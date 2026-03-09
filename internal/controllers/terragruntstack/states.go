package terragruntstack

import (
	"context"
	"fmt"
	"strings"

	configv1alpha1 "github.com/padok-team/burrito/api/v1alpha1"
	"github.com/padok-team/burrito/internal/annotations"
	"github.com/padok-team/burrito/internal/utils/syncwindow"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

type Handler func(context.Context, *Reconciler, *configv1alpha1.TerragruntStack, *configv1alpha1.TerraformRepository) (ctrl.Result, *configv1alpha1.TerragruntStackRun)

type State interface {
	getHandler() Handler
}

func (r *Reconciler) GetState(ctx context.Context, stack *configv1alpha1.TerragruntStack, repo *configv1alpha1.TerraformRepository) (State, []metav1.Condition) {
	log := log.WithContext(ctx)
	c1, isRunning := r.IsRunning(stack)
	c2, isLastPlanTooOld := r.IsLastPlanTooOld(stack)
	c3, isLastRelevantCommitPlanned := r.IsLastRelevantCommitPlanned(stack)
	c4, isSyncScheduled := r.IsSyncScheduled(stack)
	c5, isApplyScheduled := r.IsApplyScheduled(stack)
	c6, hasBlockingFailedRun := r.HasBlockingFailedRun(stack)
	conditions := []metav1.Condition{c1, c2, c3, c4, c5, c6}
	switch {
	case isRunning:
		log.Infof("stack %s is running, waiting for the run to finish", stack.Name)
		return &Idle{}, conditions
	case isApplyScheduled:
		if err := annotations.Remove(ctx, r.Client, stack, annotations.ApplyNow); err != nil {
			log.Errorf("failed to remove annotation %s from stack %s: %s", annotations.ApplyNow, stack.Name, err)
		}
		return &ApplyNeeded{}, conditions
	case isSyncScheduled:
		if err := annotations.Remove(ctx, r.Client, stack, annotations.SyncNow); err != nil {
			log.Errorf("failed to remove annotation %s from stack %s: %s", annotations.SyncNow, stack.Name, err)
		}
		return &PlanNeeded{}, conditions
	case hasBlockingFailedRun:
		log.Infof("stack %s has a blocking failed run for the current revision", stack.Name)
		return &Idle{}, conditions
	case isLastPlanTooOld || !isLastRelevantCommitPlanned:
		log.Infof("stack %s needs a plan run", stack.Name)
		return &PlanNeeded{}, conditions
	default:
		return &Idle{}, conditions
	}
}

type Idle struct{}

func (s *Idle) getHandler() Handler {
	return func(ctx context.Context, r *Reconciler, stack *configv1alpha1.TerragruntStack, repository *configv1alpha1.TerraformRepository) (ctrl.Result, *configv1alpha1.TerragruntStackRun) {
		return ctrl.Result{RequeueAfter: r.Config.Controller.Timers.DriftDetection}, nil
	}
}

type PlanNeeded struct{}

func (s *PlanNeeded) getHandler() Handler {
	return func(ctx context.Context, r *Reconciler, stack *configv1alpha1.TerragruntStack, repository *configv1alpha1.TerraformRepository) (ctrl.Result, *configv1alpha1.TerragruntStackRun) {
		log := log.WithContext(ctx)
		if isActionBlocked(r, stack, repository, syncwindow.PlanAction) {
			return ctrl.Result{RequeueAfter: r.Config.Controller.Timers.WaitAction}, nil
		}
		revision, ok := stack.Annotations[annotations.LastRelevantCommit]
		if !ok {
			r.Recorder.Event(stack, corev1.EventTypeWarning, "Reconciliation", "Stack has no last relevant commit annotation, plan run not created")
			log.Errorf("stack %s has no last relevant commit annotation, run not created", stack.Name)
			return ctrl.Result{RequeueAfter: r.Config.Controller.Timers.OnError}, nil
		}
		run := r.getRun(stack, revision, PlanAction)
		if err := r.Client.Create(ctx, &run); err != nil {
			r.Recorder.Event(stack, corev1.EventTypeWarning, "Reconciliation", "Failed to create TerragruntStackRun for Plan action")
			log.Errorf("failed to create TerragruntStackRun for Plan action on stack %s: %s", stack.Name, err)
			return ctrl.Result{RequeueAfter: r.Config.Controller.Timers.OnError}, nil
		}
		r.Recorder.Event(stack, corev1.EventTypeNormal, "Reconciliation", "Created TerragruntStackRun for Plan action")
		return ctrl.Result{RequeueAfter: r.Config.Controller.Timers.WaitAction}, &run
	}
}

type ApplyNeeded struct{}

func (s *ApplyNeeded) getHandler() Handler {
	return func(ctx context.Context, r *Reconciler, stack *configv1alpha1.TerragruntStack, repository *configv1alpha1.TerraformRepository) (ctrl.Result, *configv1alpha1.TerragruntStackRun) {
		log := log.WithContext(ctx)
		if isActionBlocked(r, stack, repository, syncwindow.ApplyAction) {
			return ctrl.Result{RequeueAfter: r.Config.Controller.Timers.WaitAction}, nil
		}
		revision, ok := stack.Annotations[annotations.LastRelevantCommit]
		if !ok {
			revision = stack.Annotations[annotations.LastBranchCommit]
		}
		if revision == "" {
			r.Recorder.Event(stack, corev1.EventTypeWarning, "Reconciliation", "Stack has no revision annotation, apply run not created")
			log.Errorf("stack %s has no revision annotation, apply run not created", stack.Name)
			return ctrl.Result{RequeueAfter: r.Config.Controller.Timers.OnError}, nil
		}
		run := r.getRun(stack, revision, ApplyAction)
		if err := r.Client.Create(ctx, &run); err != nil {
			r.Recorder.Event(stack, corev1.EventTypeWarning, "Reconciliation", "Failed to create TerragruntStackRun for Apply action")
			log.Errorf("failed to create TerragruntStackRun for Apply action on stack %s: %s", stack.Name, err)
			return ctrl.Result{RequeueAfter: r.Config.Controller.Timers.OnError}, nil
		}
		r.Recorder.Event(stack, corev1.EventTypeNormal, "Reconciliation", "Created TerragruntStackRun for Apply action")
		return ctrl.Result{RequeueAfter: r.Config.Controller.Timers.WaitAction}, &run
	}
}

func getStateString(state State) string {
	t := strings.Split(fmt.Sprintf("%T", state), ".")
	return t[len(t)-1]
}

func isActionBlocked(r *Reconciler, stack *configv1alpha1.TerragruntStack, repository *configv1alpha1.TerraformRepository, action syncwindow.Action) bool {
	defaultSyncWindows := r.Config.Controller.DefaultSyncWindows
	syncBlocked, _ := syncwindow.IsSyncBlocked(append(repository.Spec.SyncWindows, defaultSyncWindows...), action, stack.Name)
	return syncBlocked
}
