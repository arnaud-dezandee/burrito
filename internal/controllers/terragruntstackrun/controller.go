package terragruntstackrun

import (
	"bytes"
	"context"
	"io"
	"math"
	"strconv"
	"time"

	datastore "github.com/padok-team/burrito/internal/datastore/client"
	logClient "k8s.io/client-go/kubernetes"

	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	configv1alpha1 "github.com/padok-team/burrito/api/v1alpha1"
	"github.com/padok-team/burrito/internal/burrito/config"
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
	K8SLogClient *logClient.Clientset
	Scheme       *runtime.Scheme
	Config       *config.Config
	Recorder     record.EventRecorder
	Datastore    datastore.Client
	Clock
}

//+kubebuilder:rbac:groups=config.terraform.padok.cloud,resources=terragruntstackruns,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=config.terraform.padok.cloud,resources=terragruntstackruns/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=config.terraform.padok.cloud,resources=terragruntstackruns/finalizers,verbs=update

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.WithContext(ctx)
	run := &configv1alpha1.TerragruntStackRun{}
	err := r.Client.Get(ctx, req.NamespacedName, run)
	if errors.IsNotFound(err) {
		return ctrl.Result{}, nil
	}
	if err != nil {
		return ctrl.Result{}, err
	}
	if run.Status.State == "Succeeded" || run.Status.State == "Failed" {
		return ctrl.Result{}, nil
	}
	stack, err := r.getLinkedStack(run)
	if err != nil {
		return ctrl.Result{RequeueAfter: r.Config.Controller.Timers.OnError}, err
	}
	repo, err := r.getLinkedRepo(run, stack)
	if err != nil {
		return ctrl.Result{RequeueAfter: r.Config.Controller.Timers.OnError}, err
	}
	bundleOk, err := r.Datastore.CheckGitBundle(stack.Spec.Repository.Namespace, stack.Spec.Repository.Name, stack.Spec.Branch, run.Spec.Stack.Revision)
	if err != nil {
		return ctrl.Result{RequeueAfter: r.Config.Controller.Timers.OnError}, err
	}
	if !bundleOk {
		return ctrl.Result{RequeueAfter: r.Config.Controller.Timers.OnError}, nil
	}

	state, conditions := r.GetState(ctx, run, stack, repo)
	result, runInfo := state.getHandler()(ctx, r, run, stack, repo)
	if runInfo.NewPod {
		attempt := configv1alpha1.Attempt{
			PodName:      runInfo.RunnerPod,
			LogsUploaded: false,
			Number:       runInfo.Retries,
		}
		run.Status.Attempts = append(run.Status.Attempts, attempt)
	}
	run.Status = configv1alpha1.TerragruntStackRunStatus{
		Conditions:  conditions,
		State:       getStateString(state),
		Retries:     runInfo.Retries,
		LastRun:     runInfo.LastRun,
		RunnerPod:   runInfo.RunnerPod,
		Attempts:    run.Status.Attempts,
		UnitResults: run.Status.UnitResults,
	}
	if err := r.uploadLogs(run); err != nil {
		log.Errorf("failed to upload logs for stack run %s: %s", run.Name, err)
	}
	if err := r.Client.Status().Update(ctx, run); err != nil {
		log.Errorf("could not update stack run %s status: %s", run.Name, err)
	}
	return result, nil
}

func (r *Reconciler) uploadLogs(run *configv1alpha1.TerragruntStackRun) error {
	for i, attempt := range run.Status.Attempts {
		if attempt.LogsUploaded {
			continue
		}
		pod := &corev1.Pod{}
		err := r.Client.Get(context.Background(), types.NamespacedName{
			Namespace: run.Namespace,
			Name:      attempt.PodName,
		}, pod)
		if errors.IsNotFound(err) || err != nil {
			continue
		}
		if pod.Status.Phase != corev1.PodSucceeded && pod.Status.Phase != corev1.PodFailed {
			continue
		}
		req := r.K8SLogClient.CoreV1().Pods(run.Namespace).GetLogs(pod.Name, &corev1.PodLogOptions{})
		logs, err := req.Stream(context.Background())
		if err != nil {
			continue
		}
		defer logs.Close()
		buf := new(bytes.Buffer)
		if _, err := io.Copy(buf, logs); err != nil {
			continue
		}
		if err := r.Datastore.PutStackLogs(run.Namespace, run.Spec.Stack.Name, run.Name, strconv.Itoa(attempt.Number), "", buf.Bytes()); err != nil {
			return err
		}
		for _, unit := range run.Status.UnitResults {
			if err := r.Datastore.PutStackLogs(run.Namespace, run.Spec.Stack.Name, run.Name, strconv.Itoa(attempt.Number), unit.ID, buf.Bytes()); err != nil {
				return err
			}
		}
		run.Status.Attempts[i].LogsUploaded = true
	}
	return nil
}

func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.Clock = RealClock{}
	return ctrl.NewControllerManagedBy(mgr).
		For(&configv1alpha1.TerragruntStackRun{}).
		WithOptions(controller.Options{MaxConcurrentReconciles: r.Config.Controller.MaxConcurrentReconciles}).
		WithEventFilter(ignorePredicate()).
		Complete(r)
}

func GetRunExponentialBackOffTime(defaultRequeueAfter time.Duration, run *configv1alpha1.TerragruntStackRun) time.Duration {
	var attempts = run.Status.Retries
	if attempts < 1 {
		return defaultRequeueAfter
	}
	return getExponentialBackOffTime(defaultRequeueAfter, attempts)
}

func getExponentialBackOffTime(defaultRequeueAfter time.Duration, attempts int) time.Duration {
	x := float64(attempts)
	return time.Duration(int32(math.Exp(x))) * defaultRequeueAfter
}

func ignorePredicate() predicate.Predicate {
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			return (e.ObjectOld.GetGeneration() != e.ObjectNew.GetGeneration())
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return !e.DeleteStateUnknown
		},
	}
}

func (r *Reconciler) getLinkedStack(run *configv1alpha1.TerragruntStackRun) (*configv1alpha1.TerragruntStack, error) {
	stack := &configv1alpha1.TerragruntStack{}
	err := r.Client.Get(context.Background(), types.NamespacedName{
		Namespace: run.Spec.Stack.Namespace,
		Name:      run.Spec.Stack.Name,
	}, stack)
	if err != nil {
		return nil, err
	}
	return stack, nil
}

func (r *Reconciler) getLinkedRepo(run *configv1alpha1.TerragruntStackRun, stack *configv1alpha1.TerragruntStack) (*configv1alpha1.TerraformRepository, error) {
	repo := &configv1alpha1.TerraformRepository{}
	err := r.Client.Get(context.Background(), types.NamespacedName{
		Namespace: stack.Spec.Repository.Namespace,
		Name:      stack.Spec.Repository.Name,
	}, repo)
	if err != nil {
		return nil, err
	}
	return repo, nil
}
