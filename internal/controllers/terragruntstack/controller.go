package terragruntstack

import (
	"context"
	e "errors"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/padok-team/burrito/internal/burrito/config"
	datastore "github.com/padok-team/burrito/internal/datastore/client"
	"github.com/padok-team/burrito/internal/lock"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	log "github.com/sirupsen/logrus"

	configv1alpha1 "github.com/padok-team/burrito/api/v1alpha1"
)

type Clock interface {
	Now() time.Time
}

type RealClock struct{}

func (c RealClock) Now() time.Time {
	return time.Now()
}

type Reconciler struct {
	client.Client
	Scheme    *runtime.Scheme
	Config    *config.Config
	Recorder  record.EventRecorder
	Datastore datastore.Client
	Clock
}

//+kubebuilder:rbac:groups=config.terraform.padok.cloud,resources=terragruntstacks,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=config.terraform.padok.cloud,resources=terragruntstacks/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=config.terraform.padok.cloud,resources=terragruntstacks/finalizers,verbs=update

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.WithContext(ctx)
	log.Infof("starting reconciliation for stack %s/%s ...", req.Namespace, req.Name)
	stack := &configv1alpha1.TerragruntStack{}
	err := r.Client.Get(ctx, req.NamespacedName, stack)
	if errors.IsNotFound(err) {
		return ctrl.Result{}, nil
	}
	if err != nil {
		return ctrl.Result{}, err
	}
	locked, err := lock.IsStackLocked(ctx, r.Client, stack)
	if err != nil {
		return ctrl.Result{RequeueAfter: r.Config.Controller.Timers.OnError}, err
	}
	if locked {
		return ctrl.Result{RequeueAfter: r.Config.Controller.Timers.WaitAction}, nil
	}
	repository := &configv1alpha1.TerraformRepository{}
	err = r.Client.Get(ctx, types.NamespacedName{
		Namespace: stack.Spec.Repository.Namespace,
		Name:      stack.Spec.Repository.Name,
	}, repository)
	if errors.IsNotFound(err) {
		return ctrl.Result{RequeueAfter: r.Config.Controller.Timers.OnError}, err
	}
	if err != nil {
		return ctrl.Result{RequeueAfter: r.Config.Controller.Timers.OnError}, err
	}
	if err = validateStackConfig(stack, repository); err != nil {
		r.Recorder.Event(stack, corev1.EventTypeWarning, "Reconciliation", err.Error())
		stack.Status.Conditions = []metav1.Condition{{
			Type:               "ConfigValid",
			ObservedGeneration: stack.GetGeneration(),
			Status:             metav1.ConditionFalse,
			LastTransitionTime: metav1.NewTime(time.Now()),
			Reason:             "InvalidConfig",
			Message:            err.Error(),
		}}
		stack.Status.State = "Invalid"
		_ = r.Client.Status().Update(ctx, stack)
		return ctrl.Result{}, nil
	}

	state, conditions := r.GetState(ctx, stack, repository)
	lastResult := []byte("Stack has never been planned")
	if stack.Status.LastRun.Name != "" {
		lastResult, err = r.Datastore.GetStackPlan(stack.Namespace, stack.Name, stack.Status.LastRun.Name, "", "", "short")
		if err != nil {
			lastResult = []byte("Error getting last Result")
		}
	}
	result, run := state.getHandler()(ctx, r, stack, repository)
	lastRun := stack.Status.LastRun
	runHistory := stack.Status.LatestRuns
	units := stack.Status.Units
	if run != nil {
		lastRun = getRun(*run)
		runHistory = updateLatestRuns(runHistory, *run, *configv1alpha1.GetRunHistoryPolicyForStack(repository, stack).KeepLastRuns)
	}
	if stack.Status.LastRun.Name != "" {
		lastRunObj := &configv1alpha1.TerragruntStackRun{}
		if err := r.Client.Get(ctx, types.NamespacedName{Namespace: stack.Namespace, Name: stack.Status.LastRun.Name}, lastRunObj); err == nil {
			units = mergeUnitResults(units, lastRunObj.Status.UnitResults, lastRunObj.Status.State != "Succeeded" && lastRunObj.Status.State != "Failed")
		}
	}
	stack.Status = configv1alpha1.TerragruntStackStatus{Conditions: conditions, State: getStateString(state), LastResult: string(lastResult), LastRun: lastRun, LatestRuns: runHistory, Units: units}
	if err := r.Client.Status().Update(ctx, stack); err != nil {
		log.Errorf("could not update stack %s status: %s", stack.Name, err)
	}
	if err := r.cleanupRuns(ctx, stack, repository); err != nil {
		log.Warningf("failed to cleanup runs for stack %s: %s", stack.Name, err)
	}
	return result, nil
}

func (r *Reconciler) cleanupRuns(ctx context.Context, stack *configv1alpha1.TerragruntStack, repository *configv1alpha1.TerraformRepository) error {
	historyPolicy := configv1alpha1.GetRunHistoryPolicyForStack(repository, stack)
	runs, err := r.getAllRuns(ctx, stack)
	if len(runs) < *historyPolicy.KeepLastRuns {
		return nil
	}
	if err != nil {
		return err
	}
	runsToKeep := map[string]bool{}
	for _, run := range stack.Status.LatestRuns {
		runsToKeep[run.Name] = true
	}
	toDelete := []*configv1alpha1.TerragruntStackRun{}
	for _, run := range runs {
		if _, ok := runsToKeep[run.Name]; !ok {
			toDelete = append(toDelete, run)
		}
	}
	if len(toDelete) == 0 {
		return nil
	}
	return deleteAll(ctx, r.Client, toDelete)
}

func getRun(run configv1alpha1.TerragruntStackRun) configv1alpha1.TerragruntStackRunRef {
	return configv1alpha1.TerragruntStackRunRef{
		Name:   run.Name,
		Commit: run.Spec.Stack.Revision,
		Date:   run.CreationTimestamp,
		Action: run.Spec.Action,
	}
}

func updateLatestRuns(runs []configv1alpha1.TerragruntStackRunRef, run configv1alpha1.TerragruntStackRun, keep int) []configv1alpha1.TerragruntStackRunRef {
	oldestRun := &configv1alpha1.TerragruntStackRunRef{
		Date: metav1.NewTime(time.Now()),
	}
	var oldestRunIndex int
	newRun := getRun(run)
	for i, r := range runs {
		if r.Date.Before(&oldestRun.Date) {
			oldestRun = &r
			oldestRunIndex = i
		}
	}
	if oldestRun == nil || len(runs) < keep {
		return append(runs, newRun)
	}
	rs := append(runs[:oldestRunIndex], runs[oldestRunIndex+1:]...)
	return append(rs, newRun)
}

func mergeUnitResults(units []configv1alpha1.TerragruntStackUnit, results []configv1alpha1.TerragruntStackUnitResult, isRunning bool) []configv1alpha1.TerragruntStackUnit {
	index := map[string]*configv1alpha1.TerragruntStackUnit{}
	for i := range units {
		index[units[i].ID] = &units[i]
	}
	for _, result := range results {
		unit, ok := index[result.ID]
		if !ok {
			units = append(units, configv1alpha1.TerragruntStackUnit{ID: result.ID, Path: result.Path})
			unit = &units[len(units)-1]
			index[result.ID] = unit
		}
		unit.Path = result.Path
		unit.State = result.State
		unit.LastAction = result.Action
		unit.LastRun = result.Run
		unit.LastRunAt = result.RunAt
		unit.LastResult = result.Result
		unit.HasValidPlan = result.HasValidPlan
		unit.LastPlannedRevision = result.LastPlannedRevision
		unit.LastAppliedRevision = result.LastAppliedRevision
		unit.IsRunning = isRunning
		latestRun := configv1alpha1.TerragruntUnitRunRef{
			Run:    result.Run,
			Action: result.Action,
			Date:   result.RunAt,
		}
		if len(unit.LatestRuns) == 0 || unit.LatestRuns[len(unit.LatestRuns)-1] != latestRun {
			unit.LatestRuns = append(unit.LatestRuns, latestRun)
		}
	}
	return units
}

func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.Clock = RealClock{}
	return ctrl.NewControllerManagedBy(mgr).
		For(&configv1alpha1.TerragruntStack{}).
		WithOptions(controller.Options{MaxConcurrentReconciles: r.Config.Controller.MaxConcurrentReconciles}).
		WithEventFilter(ignorePredicate()).
		Complete(r)
}

func ignorePredicate() predicate.Predicate {
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			return (e.ObjectOld.GetGeneration() != e.ObjectNew.GetGeneration()) ||
				cmp.Diff(e.ObjectOld.GetAnnotations(), e.ObjectNew.GetAnnotations()) != ""
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return !e.DeleteStateUnknown
		},
	}
}

func validateStackConfig(stack *configv1alpha1.TerragruntStack, repository *configv1alpha1.TerraformRepository) error {
	if !configv1alpha1.GetTerragruntEnabledForStack(repository, stack) {
		return e.New("TerragruntStack configuration is invalid: Terragrunt must be enabled for this stack")
	}
	if !configv1alpha1.GetTerraformEnabledForStack(repository, stack) && !configv1alpha1.GetOpenTofuEnabledForStack(repository, stack) {
		return e.New("TerragruntStack configuration is invalid: Neither Terraform nor OpenTofu is enabled for this stack")
	}
	if configv1alpha1.GetAutoApplyEnabledForStack(repository, stack) {
		return e.New("TerragruntStack configuration is invalid: autoApply is not supported for TerragruntStack")
	}
	return nil
}
