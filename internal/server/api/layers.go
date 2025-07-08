package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	configv1alpha1 "github.com/padok-team/burrito/api/v1alpha1"
	"github.com/padok-team/burrito/internal/annotations"
	"github.com/padok-team/burrito/internal/server/utils"
	log "github.com/sirupsen/logrus"
)

type layer struct {
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
	IsPR             bool                   `json:"isPR"`
	LatestRuns       []Run                  `json:"latestRuns"`
	ManualSyncStatus utils.ManualSyncStatus `json:"manualSyncStatus"`
	HasValidPlan     bool                   `json:"hasValidPlan"`
	AutoApply        bool                   `json:"autoApply"`
}

type Run struct {
	Name   string `json:"id"`
	Commit string `json:"commit"`
	Date   string `json:"date"`
	Action string `json:"action"`
}

type layersResponse struct {
	Results []layer `json:"results"`
}

func (a *API) getLayersAndRuns() ([]configv1alpha1.TerraformLayer, map[string]configv1alpha1.TerraformRun, map[string]configv1alpha1.TerraformRepository, error) {
	layers := &configv1alpha1.TerraformLayerList{}
	err := a.Client.List(context.Background(), layers)
	if err != nil {
		log.Errorf("could not list TerraformLayers: %s", err)
		return nil, nil, nil, err
	}
	runs := &configv1alpha1.TerraformRunList{}
	indexedRuns := map[string]configv1alpha1.TerraformRun{}
	err = a.Client.List(context.Background(), runs)
	if err != nil {
		log.Errorf("could not list TerraformRuns: %s", err)
		return nil, nil, nil, err
	}
	for _, run := range runs.Items {
		indexedRuns[fmt.Sprintf("%s/%s", run.Namespace, run.Name)] = run
	}
	repositories := &configv1alpha1.TerraformRepositoryList{}
	indexedRepositories := map[string]configv1alpha1.TerraformRepository{}
	err = a.Client.List(context.Background(), repositories)
	if err != nil {
		log.Errorf("could not list TerraformRepositories: %s", err)
		return nil, nil, nil, err
	}
	for _, repo := range repositories.Items {
		indexedRepositories[fmt.Sprintf("%s/%s", repo.Namespace, repo.Name)] = repo
	}
	return layers.Items, indexedRuns, indexedRepositories, err
}

func (a *API) buildLayersResponse(layersData []configv1alpha1.TerraformLayer, runs map[string]configv1alpha1.TerraformRun, repositories map[string]configv1alpha1.TerraformRepository) []layer {
	results := []layer{}
	for _, l := range layersData {
		run, ok := runs[fmt.Sprintf("%s/%s", l.Namespace, l.Status.LastRun.Name)]
		runAPI := Run{}
		running := false
		if ok {
			runAPI = Run{
				Name:   run.Name,
				Commit: "",
				Date:   run.CreationTimestamp.Format(time.RFC3339),
				Action: run.Spec.Action,
			}
			running = runStillRunning(run)
		}

		// Get repository for this layer to calculate AutoApply
		repoKey := fmt.Sprintf("%s/%s", l.Spec.Repository.Namespace, l.Spec.Repository.Name)
		repo, repoExists := repositories[repoKey]
		autoApply := false
		if repoExists {
			autoApply = configv1alpha1.GetAutoApplyEnabled(&repo, &l)
		}

		results = append(results, layer{
			UID:              string(l.UID),
			Name:             l.Name,
			Namespace:        l.Namespace,
			Repository:       fmt.Sprintf("%s/%s", l.Spec.Repository.Namespace, l.Spec.Repository.Name),
			Branch:           l.Spec.Branch,
			Path:             l.Spec.Path,
			State:            a.getLayerState(l),
			RunCount:         len(l.Status.LatestRuns),
			LastRun:          runAPI,
			LastRunAt:        l.Status.LastRun.Date.Format(time.RFC3339),
			LastResult:       l.Status.LastResult,
			IsRunning:        running,
			IsPR:             a.isLayerPR(l),
			LatestRuns:       transformLatestRuns(l.Status.LatestRuns),
			ManualSyncStatus: getManualOperationStatus(l),
			HasValidPlan:     hasValidPlan(l),
			AutoApply:        autoApply,
		})
	}
	return results
}

func (a *API) LayersHandler(c echo.Context) error {
	layers, runs, repositories, err := a.getLayersAndRuns()
	if err != nil {
		return c.String(http.StatusInternalServerError, fmt.Sprintf("could not list terraform layers or runs: %s", err))
	}

	results := a.buildLayersResponse(layers, runs, repositories)

	return c.JSON(http.StatusOK, &layersResponse{
		Results: results,
	})
}

func runStillRunning(run configv1alpha1.TerraformRun) bool {
	if run.Status.State != "Failed" && run.Status.State != "Succeeded" {
		return true
	}
	return false
}

func transformLatestRuns(runs []configv1alpha1.TerraformLayerRun) []Run {
	results := []Run{}
	for _, r := range runs {
		results = append(results, Run{
			Name:   r.Name,
			Commit: r.Commit,
			Date:   r.Date.Format(time.RFC3339),
			Action: r.Action,
		})
	}
	return results
}

func (a *API) getLayerState(layer configv1alpha1.TerraformLayer) string {
	state := "success"
	switch {
	case len(layer.Status.Conditions) == 0:
		state = "disabled"
	case layer.Status.State == "ApplyNeeded":
		if layer.Status.LastResult == "Plan: 0 to create, 0 to update, 0 to delete" {
			state = "success"
		} else {
			state = "warning"
		}
	case layer.Status.State == "PlanNeeded":
		state = "warning"
	}
	if layer.Annotations[annotations.LastPlanSum] == "" {
		state = "error"
	}
	if layer.Annotations[annotations.LastApplySum] != "" && layer.Annotations[annotations.LastApplySum] == "" {
		state = "error"
	}
	return state
}

func (a *API) isLayerPR(layer configv1alpha1.TerraformLayer) bool {
	if len(layer.OwnerReferences) == 0 {
		return false
	}
	return layer.OwnerReferences[0].Kind == "TerraformPullRequest"
}

func getManualOperationStatus(layer configv1alpha1.TerraformLayer) utils.ManualSyncStatus {
	// Check apply status first, then sync status
	applyStatus := utils.GetManualApplyStatus(layer)
	if applyStatus != utils.ManualSyncNone {
		return applyStatus
	}
	return utils.GetManualSyncStatus(layer)
}

func hasValidPlan(layer configv1alpha1.TerraformLayer) bool {
	// A valid plan exists if LastPlanSum annotation exists and is not empty
	// This matches the logic in HasLastPlanFailed condition
	planSum, exists := layer.Annotations[annotations.LastPlanSum]
	return exists && planSum != ""
}

func (a *API) LayersEventsHandler(c echo.Context) error {
	// Set headers for Server-Sent Events
	c.Response().Header().Set("Content-Type", "text/event-stream")
	c.Response().Header().Set("Cache-Control", "no-cache")
	c.Response().Header().Set("Connection", "keep-alive")
	c.Response().Header().Set("Access-Control-Allow-Origin", "*")
	c.Response().Header().Set("Access-Control-Allow-Headers", "Cache-Control")

	ctx := c.Request().Context()

	// Create a channel for this client
	clientChan := make(chan []byte, 10)
	clientID := fmt.Sprintf("client-%d", time.Now().UnixNano())

	// Subscribe to watch manager
	if a.watchManager != nil {
		a.watchManager.Subscribe(clientID, clientChan)
		defer a.watchManager.Unsubscribe(clientID)
	} else {
		log.Error("Watch manager not initialized")
		return c.String(http.StatusInternalServerError, "Watch manager not available")
	}

	// Send initial data
	layersData, runs, repositories, err := a.getLayersAndRuns()
	if err != nil {
		log.Errorf("failed to get initial layers data: %s", err)
		return c.String(http.StatusInternalServerError, "Failed to get layers data")
	}

	results := a.buildLayersResponse(layersData, runs, repositories)

	initialData, err := json.Marshal(&layersResponse{Results: results})
	if err != nil {
		log.Errorf("failed to marshal initial data: %s", err)
		return c.String(http.StatusInternalServerError, "Failed to marshal data")
	}

	fmt.Fprintf(c.Response(), "data: %s\n\n", initialData)
	c.Response().Flush()

	// Listen for updates
	for {
		select {
		case data := <-clientChan:
			fmt.Fprintf(c.Response(), "data: %s\n\n", data)
			c.Response().Flush()
		case <-ctx.Done():
			log.Info("client disconnected")
			return nil
		}
	}
}

func (a *API) GetPlanHandler(c echo.Context) error {
	namespace := c.Param("namespace")
	layer := c.Param("layer")
	run := c.Param("run")
	attempt := c.Param("attempt")

	if namespace == "" || layer == "" || run == "" || attempt == "" {
		return c.String(http.StatusBadRequest, "Missing required parameters: namespace, layer, run, attempt")
	}

	planData, err := a.Datastore.GetPlan(namespace, layer, run, attempt, "pretty")
	if err != nil {
		log.Errorf("could not get plan for %s/%s/%s/%s: %s", namespace, layer, run, attempt, err)
		return c.String(http.StatusInternalServerError, fmt.Sprintf("could not get plan: %s", err))
	}

	if planData == nil {
		return c.String(http.StatusNotFound, "Plan not found")
	}

	c.Response().Header().Set("Content-Type", "text/plain; charset=utf-8")
	return c.String(http.StatusOK, string(planData))
}
