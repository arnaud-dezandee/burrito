package terragruntstackrun

import (
	"context"
	"fmt"
	"strings"
	"time"

	configv1alpha1 "github.com/padok-team/burrito/api/v1alpha1"
	"github.com/padok-team/burrito/internal/lock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

type RunInfo struct {
	Retries   int
	LastRun   string
	RunnerPod string
	NewPod    bool
}

func getRunInfo(run *configv1alpha1.TerragruntStackRun) RunInfo {
	return RunInfo{Retries: run.Status.Retries, LastRun: run.Status.LastRun, RunnerPod: run.Status.RunnerPod}
}

type Handler func(context.Context, *Reconciler, *configv1alpha1.TerragruntStackRun, *configv1alpha1.TerragruntStack, *configv1alpha1.TerraformRepository) (ctrl.Result, RunInfo)

type State interface {
	getHandler() Handler
}

func (r *Reconciler) GetState(ctx context.Context, run *configv1alpha1.TerragruntStackRun, stack *configv1alpha1.TerragruntStack, repo *configv1alpha1.TerraformRepository) (State, []metav1.Condition) {
	c1, hasStatus := r.HasStatus(run)
	c2, hasReachedRetryLimit := r.HasReachedRetryLimit(run, stack, repo)
	c3, hasSucceeded := r.HasSucceeded(run)
	c4, isRunning := r.IsRunning(run)
	c5, isInFailureGracePeriod := r.IsInFailureGracePeriod(run)
	conditions := []metav1.Condition{c1, c2, c3, c4, c5}
	switch {
	case !hasStatus:
		return &Initial{}, conditions
	case hasSucceeded:
		return &Succeeded{}, conditions
	case isInFailureGracePeriod && !hasReachedRetryLimit && !isRunning:
		return &FailureGracePeriod{}, conditions
	case isInFailureGracePeriod && hasReachedRetryLimit && !isRunning:
		return &Failed{}, conditions
	case !isRunning && !hasReachedRetryLimit:
		return &Retrying{}, conditions
	case isRunning:
		return &Running{}, conditions
	default:
		return &Failed{}, conditions
	}
}

type Initial struct{}

func (s *Initial) getHandler() Handler {
	return func(ctx context.Context, r *Reconciler, run *configv1alpha1.TerragruntStackRun, stack *configv1alpha1.TerragruntStack, repo *configv1alpha1.TerraformRepository) (ctrl.Result, RunInfo) {
		if err := lock.CreateStackLock(ctx, r.Client, stack, run); err != nil {
			return ctrl.Result{RequeueAfter: r.Config.Controller.Timers.OnError}, RunInfo{}
		}
		pod := r.getPod(run, stack, repo)
		if err := r.Client.Create(ctx, &pod); err != nil {
			return ctrl.Result{RequeueAfter: r.Config.Controller.Timers.OnError}, RunInfo{}
		}
		return ctrl.Result{RequeueAfter: time.Second}, RunInfo{
			Retries:   0,
			LastRun:   r.Clock.Now().Format(time.UnixDate),
			RunnerPod: pod.Name,
			NewPod:    true,
		}
	}
}

type Running struct{}

func (s *Running) getHandler() Handler {
	return func(ctx context.Context, r *Reconciler, run *configv1alpha1.TerragruntStackRun, stack *configv1alpha1.TerragruntStack, repo *configv1alpha1.TerraformRepository) (ctrl.Result, RunInfo) {
		return ctrl.Result{RequeueAfter: r.Config.Controller.Timers.WaitAction}, getRunInfo(run)
	}
}

type FailureGracePeriod struct{}

func (s *FailureGracePeriod) getHandler() Handler {
	return func(ctx context.Context, r *Reconciler, run *configv1alpha1.TerragruntStackRun, stack *configv1alpha1.TerragruntStack, repo *configv1alpha1.TerraformRepository) (ctrl.Result, RunInfo) {
		lastActionTime, ok := getLastActionTime(r, run)
		if ok != nil {
			return ctrl.Result{RequeueAfter: r.Config.Controller.Timers.OnError}, getRunInfo(run)
		}
		expTime := GetRunExponentialBackOffTime(r.Config.Controller.Timers.FailureGracePeriod, run)
		endIdleTime := lastActionTime.Add(expTime)
		now := r.Clock.Now()
		if endIdleTime.After(now) {
			return ctrl.Result{RequeueAfter: r.Config.Controller.Timers.WaitAction}, getRunInfo(run)
		}
		return ctrl.Result{RequeueAfter: now.Sub(endIdleTime)}, getRunInfo(run)
	}
}

type Retrying struct{}

func (s *Retrying) getHandler() Handler {
	return func(ctx context.Context, r *Reconciler, run *configv1alpha1.TerragruntStackRun, stack *configv1alpha1.TerragruntStack, repo *configv1alpha1.TerraformRepository) (ctrl.Result, RunInfo) {
		pod := r.getPod(run, stack, repo)
		if err := r.Client.Create(ctx, &pod); err != nil {
			return ctrl.Result{RequeueAfter: r.Config.Controller.Timers.OnError}, getRunInfo(run)
		}
		return ctrl.Result{RequeueAfter: time.Second}, RunInfo{
			Retries:   run.Status.Retries + 1,
			LastRun:   r.Clock.Now().Format(time.UnixDate),
			RunnerPod: pod.Name,
			NewPod:    true,
		}
	}
}

type Succeeded struct{}

func (s *Succeeded) getHandler() Handler {
	return func(ctx context.Context, r *Reconciler, run *configv1alpha1.TerragruntStackRun, stack *configv1alpha1.TerragruntStack, repo *configv1alpha1.TerraformRepository) (ctrl.Result, RunInfo) {
		if err := lock.DeleteStackLock(ctx, r.Client, stack, run); err != nil {
			return ctrl.Result{RequeueAfter: r.Config.Controller.Timers.OnError}, getRunInfo(run)
		}
		return ctrl.Result{RequeueAfter: r.Config.Controller.Timers.WaitAction}, getRunInfo(run)
	}
}

type Failed struct{}

func (s *Failed) getHandler() Handler {
	return func(ctx context.Context, r *Reconciler, run *configv1alpha1.TerragruntStackRun, stack *configv1alpha1.TerragruntStack, repo *configv1alpha1.TerraformRepository) (ctrl.Result, RunInfo) {
		if err := lock.DeleteStackLock(ctx, r.Client, stack, run); err != nil {
			return ctrl.Result{RequeueAfter: r.Config.Controller.Timers.OnError}, getRunInfo(run)
		}
		return ctrl.Result{RequeueAfter: r.Config.Controller.Timers.WaitAction}, getRunInfo(run)
	}
}

func getStateString(state State) string {
	t := strings.Split(fmt.Sprintf("%T", state), ".")
	return t[len(t)-1]
}
