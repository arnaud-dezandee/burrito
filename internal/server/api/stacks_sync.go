package api

import (
	"context"
	"net/http"

	"github.com/labstack/echo/v4"
	configv1alpha1 "github.com/padok-team/burrito/api/v1alpha1"
	"github.com/padok-team/burrito/internal/annotations"
	"github.com/padok-team/burrito/internal/server/utils"
	log "github.com/sirupsen/logrus"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (a *API) SyncStackHandler(c echo.Context) error {
	stack := &configv1alpha1.TerragruntStack{}
	err := a.Client.Get(context.Background(), client.ObjectKey{
		Namespace: c.Param("namespace"),
		Name:      c.Param("stack"),
	}, stack)
	if err != nil {
		log.Errorf("could not get terragrunt stack: %s", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "An error occurred while getting the stack"})
	}
	syncStatus := utils.GetManualSyncStatusGeneric(stack.Annotations)
	if syncStatus == utils.ManualSyncAnnotated || syncStatus == utils.ManualSyncPending {
		return c.JSON(http.StatusConflict, map[string]string{"error": "Stack sync already triggered"})
	}
	if err := annotations.Add(context.Background(), a.Client, stack, map[string]string{annotations.SyncNow: "true"}); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "An error occurred while updating the stack annotations"})
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "Stack sync triggered"})
}

func (a *API) ApplyStackHandler(c echo.Context) error {
	stack := &configv1alpha1.TerragruntStack{}
	err := a.Client.Get(context.Background(), client.ObjectKey{
		Namespace: c.Param("namespace"),
		Name:      c.Param("stack"),
	}, stack)
	if err != nil {
		log.Errorf("could not get terragrunt stack: %s", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "An error occurred while getting the stack"})
	}
	if err := annotations.Add(context.Background(), a.Client, stack, map[string]string{annotations.ApplyNow: "true"}); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "An error occurred while updating the stack annotations"})
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "Stack apply triggered"})
}
