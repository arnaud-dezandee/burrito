package terragruntstack

import (
	"context"
	"fmt"
	"time"

	configv1alpha1 "github.com/padok-team/burrito/api/v1alpha1"
	"github.com/padok-team/burrito/internal/annotations"
	"github.com/padok-team/burrito/internal/utils/pathmatcher"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func (r *Reconciler) IsRunning(stack *configv1alpha1.TerragruntStack) (metav1.Condition, bool) {
	condition := metav1.Condition{
		Type:               "IsRunning",
		ObservedGeneration: stack.GetObjectMeta().GetGeneration(),
		Status:             metav1.ConditionUnknown,
		LastTransitionTime: metav1.NewTime(time.Now()),
	}
	if stack.Status.LastRun.Name == "" {
		condition.Reason = "NoRunHasRunYet"
		condition.Message = "No run has run on this stack yet"
		condition.Status = metav1.ConditionFalse
		return condition, false
	}
	run := configv1alpha1.TerragruntStackRun{}
	err := r.Client.Get(context.TODO(), types.NamespacedName{
		Namespace: stack.Namespace,
		Name:      stack.Status.LastRun.Name,
	}, &run)
	if errors.IsNotFound(err) {
		condition.Reason = "RunNotFound"
		condition.Message = "The last run could not be fetched, considering stack is not running"
		condition.Status = metav1.ConditionFalse
		return condition, false
	}
	if err != nil {
		condition.Reason = "RunRetrievalError"
		condition.Message = "An error happened while fetching the last run, considering stack is running"
		condition.Status = metav1.ConditionTrue
		return condition, true
	}
	if run.Status.State != "Succeeded" && run.Status.State != "Failed" {
		condition.Reason = "RunStillRunning"
		condition.Message = "The last run is still running"
		condition.Status = metav1.ConditionTrue
		return condition, true
	}
	condition.Reason = "RunFinished"
	condition.Message = "The last run has finished"
	condition.Status = metav1.ConditionFalse
	return condition, false
}

func (r *Reconciler) IsLastPlanTooOld(stack *configv1alpha1.TerragruntStack) (metav1.Condition, bool) {
	condition := metav1.Condition{
		Type:               "IsLastPlanTooOld",
		ObservedGeneration: stack.GetObjectMeta().GetGeneration(),
		Status:             metav1.ConditionUnknown,
		LastTransitionTime: metav1.NewTime(time.Now()),
	}
	value, ok := stack.Annotations[annotations.LastPlanDate]
	if !ok {
		condition.Reason = "NoPlanHasRunYet"
		condition.Message = "No plan has run on this stack yet"
		condition.Status = metav1.ConditionTrue
		return condition, true
	}
	lastPlanDate, err := time.Parse(time.UnixDate, value)
	if err != nil {
		condition.Reason = "ParseError"
		condition.Message = "Could not parse stack plan date annotation"
		condition.Status = metav1.ConditionFalse
		return condition, false
	}
	nextPlanDate := lastPlanDate.Add(r.Config.Controller.Timers.DriftDetection)
	if nextPlanDate.After(r.Clock.Now()) {
		condition.Reason = "PlanIsRecent"
		condition.Message = "The plan is recent"
		condition.Status = metav1.ConditionFalse
		return condition, false
	}
	condition.Reason = "PlanIsTooOld"
	condition.Message = "The plan is too old"
	condition.Status = metav1.ConditionTrue
	return condition, true
}

func (r *Reconciler) IsLastRelevantCommitPlanned(stack *configv1alpha1.TerragruntStack) (metav1.Condition, bool) {
	condition := metav1.Condition{
		Type:               "IsLastRelevantCommitPlanned",
		ObservedGeneration: stack.GetObjectMeta().GetGeneration(),
		Status:             metav1.ConditionUnknown,
		LastTransitionTime: metav1.NewTime(time.Now()),
	}
	lastPlannedCommit, ok := stack.Annotations[annotations.LastPlanCommit]
	if !ok {
		condition.Reason = "NoPlanHasRunYet"
		condition.Message = "No plan has run on this stack yet"
		condition.Status = metav1.ConditionFalse
		return condition, false
	}
	lastRelevantCommit, ok := stack.Annotations[annotations.LastRelevantCommit]
	if !ok {
		condition.Reason = "NoRelevantCommitReceived"
		condition.Message = "No relevant commit has been received from webhook"
		condition.Status = metav1.ConditionFalse
		return condition, false
	}
	if lastPlannedCommit == lastRelevantCommit {
		condition.Reason = "LastRelevantCommitPlanned"
		condition.Message = "The last relevant commit has already been planned"
		condition.Status = metav1.ConditionTrue
		return condition, true
	}
	condition.Reason = "LastRelevantCommitNotPlanned"
	condition.Message = "The last relevant commit has not been planned yet"
	condition.Status = metav1.ConditionFalse
	return condition, false
}

func (r *Reconciler) IsSyncScheduled(stack *configv1alpha1.TerragruntStack) (metav1.Condition, bool) {
	condition := metav1.Condition{
		Type:               "IsSyncScheduled",
		ObservedGeneration: stack.GetObjectMeta().GetGeneration(),
		Status:             metav1.ConditionUnknown,
		LastTransitionTime: metav1.NewTime(time.Now()),
	}
	if _, ok := stack.Annotations[annotations.SyncNow]; ok {
		condition.Reason = "SyncScheduled"
		condition.Message = "A sync has been manually scheduled"
		condition.Status = metav1.ConditionTrue
		return condition, true
	}
	condition.Reason = "NoSyncScheduled"
	condition.Message = "No sync has been manually scheduled"
	condition.Status = metav1.ConditionFalse
	return condition, false
}

func (r *Reconciler) IsApplyScheduled(stack *configv1alpha1.TerragruntStack) (metav1.Condition, bool) {
	condition := metav1.Condition{
		Type:               "IsApplyScheduled",
		ObservedGeneration: stack.GetObjectMeta().GetGeneration(),
		Status:             metav1.ConditionUnknown,
		LastTransitionTime: metav1.NewTime(time.Now()),
	}
	if _, ok := stack.Annotations[annotations.ApplyNow]; ok {
		condition.Reason = "ApplyScheduled"
		condition.Message = "An apply has been manually scheduled"
		condition.Status = metav1.ConditionTrue
		return condition, true
	}
	condition.Reason = "NoApplyScheduled"
	condition.Message = "No apply has been manually scheduled"
	condition.Status = metav1.ConditionFalse
	return condition, false
}

func (r *Reconciler) HasBlockingFailedRun(stack *configv1alpha1.TerragruntStack) (metav1.Condition, bool) {
	condition := metav1.Condition{
		Type:               "HasBlockingFailedRun",
		ObservedGeneration: stack.GetObjectMeta().GetGeneration(),
		Status:             metav1.ConditionUnknown,
		LastTransitionTime: metav1.NewTime(time.Now()),
	}
	if stack.Status.LastRun.Name == "" {
		condition.Reason = "NoRunYet"
		condition.Message = "No run has been created for this stack yet"
		condition.Status = metav1.ConditionFalse
		return condition, false
	}
	run := configv1alpha1.TerragruntStackRun{}
	err := r.Client.Get(context.TODO(), types.NamespacedName{
		Namespace: stack.Namespace,
		Name:      stack.Status.LastRun.Name,
	}, &run)
	if err != nil {
		condition.Reason = "RunRetrievalError"
		condition.Message = "Could not fetch the last run"
		condition.Status = metav1.ConditionFalse
		return condition, false
	}
	if run.Status.State != "Failed" {
		condition.Reason = "LastRunNotFailed"
		condition.Message = "The last run has not failed"
		condition.Status = metav1.ConditionFalse
		return condition, false
	}
	currentRevision := stack.Annotations[annotations.LastRelevantCommit]
	if currentRevision == "" || run.Spec.Stack.Revision != currentRevision {
		condition.Reason = "NewRevisionAvailable"
		condition.Message = "The last run failed but a new revision is available"
		condition.Status = metav1.ConditionFalse
		return condition, false
	}
	condition.Reason = "CurrentRevisionFailed"
	condition.Message = fmt.Sprintf("The last %s run failed for the current revision", run.Spec.Action)
	condition.Status = metav1.ConditionTrue
	return condition, true
}

func StackFilesHaveChanged(stack configv1alpha1.TerragruntStack, changedFiles []string) bool {
	return pathmatcher.FilesHaveChanged(stack.Spec.Path, stack.Annotations, changedFiles)
}
