package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	configv1alpha1 "github.com/padok-team/burrito/api/v1alpha1"
	"github.com/padok-team/burrito/internal/annotations"
	"github.com/padok-team/burrito/internal/server/utils"
	log "github.com/sirupsen/logrus"
)

type stack struct {
	UID              string                 `json:"uid"`
	Name             string                 `json:"name"`
	Namespace        string                 `json:"namespace"`
	Repository       string                 `json:"repository"`
	Branch           string                 `json:"branch"`
	Path             string                 `json:"path"`
	State            string                 `json:"state"`
	RunCount         int                    `json:"runCount"`
	LastRun          Run                    `json:"lastRun"`
	LastRunAt        string                 `json:"lastRunAt"`
	LastResult       string                 `json:"lastResult"`
	IsRunning        bool                   `json:"isRunning"`
	LatestRuns       []Run                  `json:"latestRuns"`
	ManualSyncStatus utils.ManualSyncStatus `json:"manualSyncStatus"`
	Units            []stackUnit            `json:"units"`
}

type stackUnit struct {
	ID                  string `json:"id"`
	Path                string `json:"path"`
	State               string `json:"state"`
	LastAction          string `json:"lastAction"`
	LastRunAt           string `json:"lastRunAt"`
	LastResult          string `json:"lastResult"`
	HasValidPlan        bool   `json:"hasValidPlan"`
	LastPlannedRevision string `json:"lastPlannedRevision"`
	LastAppliedRevision string `json:"lastAppliedRevision"`
	IsRunning           bool   `json:"isRunning"`
}

type stacksResponse struct {
	Results []stack `json:"results"`
}

func (a *API) getStacksAndRuns() ([]configv1alpha1.TerragruntStack, map[string]configv1alpha1.TerragruntStackRun, error) {
	stacks := &configv1alpha1.TerragruntStackList{}
	if err := a.Client.List(context.Background(), stacks); err != nil {
		log.Errorf("could not list TerragruntStacks: %s", err)
		return nil, nil, err
	}
	runs := &configv1alpha1.TerragruntStackRunList{}
	indexedRuns := map[string]configv1alpha1.TerragruntStackRun{}
	if err := a.Client.List(context.Background(), runs); err != nil {
		log.Errorf("could not list TerragruntStackRuns: %s", err)
		return nil, nil, err
	}
	for _, run := range runs.Items {
		indexedRuns[fmt.Sprintf("%s/%s", run.Namespace, run.Name)] = run
	}
	return stacks.Items, indexedRuns, nil
}

func (a *API) StacksHandler(c echo.Context) error {
	stacks, runs, err := a.getStacksAndRuns()
	if err != nil {
		return c.String(http.StatusInternalServerError, fmt.Sprintf("could not list terragrunt stacks or runs: %s", err))
	}
	results := []stack{}
	for _, s := range stacks {
		run, ok := runs[fmt.Sprintf("%s/%s", s.Namespace, s.Status.LastRun.Name)]
		runAPI := Run{}
		running := false
		if ok {
			runAPI = Run{Name: run.Name, Commit: run.Spec.Stack.Revision, Date: run.CreationTimestamp.Format(time.RFC3339), Action: run.Spec.Action}
			running = runStillRunningStack(run)
		}
		units := []stackUnit{}
		for _, unit := range s.Status.Units {
			units = append(units, stackUnit{
				ID:                  unit.ID,
				Path:                unit.Path,
				State:               unit.State,
				LastAction:          unit.LastAction,
				LastRunAt:           unit.LastRunAt.Format(time.RFC3339),
				LastResult:          unit.LastResult,
				HasValidPlan:        unit.HasValidPlan,
				LastPlannedRevision: unit.LastPlannedRevision,
				LastAppliedRevision: unit.LastAppliedRevision,
				IsRunning:           unit.IsRunning,
			})
		}
		results = append(results, stack{
			UID:              string(s.UID),
			Name:             s.Name,
			Namespace:        s.Namespace,
			Repository:       fmt.Sprintf("%s/%s", s.Spec.Repository.Namespace, s.Spec.Repository.Name),
			Branch:           s.Spec.Branch,
			Path:             s.Spec.Path,
			State:            getStackState(s),
			RunCount:         len(s.Status.LatestRuns),
			LastRun:          runAPI,
			LastRunAt:        s.Status.LastRun.Date.Format(time.RFC3339),
			LastResult:       s.Status.LastResult,
			IsRunning:        running,
			LatestRuns:       transformStackLatestRuns(s.Status.LatestRuns),
			ManualSyncStatus: getManualOperationStatusForStack(s),
			Units:            units,
		})
	}
	return c.JSON(http.StatusOK, &stacksResponse{Results: results})
}

func runStillRunningStack(run configv1alpha1.TerragruntStackRun) bool {
	return run.Status.State != "Failed" && run.Status.State != "Succeeded"
}

func transformStackLatestRuns(runs []configv1alpha1.TerragruntStackRunRef) []Run {
	results := []Run{}
	for _, r := range runs {
		results = append(results, Run{Name: r.Name, Commit: r.Commit, Date: r.Date.Format(time.RFC3339), Action: r.Action})
	}
	return results
}

func getStackState(stack configv1alpha1.TerragruntStack) string {
	state := "success"
	switch {
	case len(stack.Status.Conditions) == 0:
		state = "disabled"
	case stack.Status.State == "PlanNeeded" || stack.Status.State == "ApplyNeeded":
		state = "warning"
	}
	if stack.Annotations[annotations.LastPlanSum] == "" && len(stack.Status.Units) == 0 {
		state = "error"
	}
	return state
}

func getManualOperationStatusForStack(stack configv1alpha1.TerragruntStack) utils.ManualSyncStatus {
	applyStatus := utils.GetManualApplyStatusGeneric(stack.Annotations)
	if applyStatus != utils.ManualSyncNone {
		return applyStatus
	}
	return utils.GetManualSyncStatusGeneric(stack.Annotations)
}
