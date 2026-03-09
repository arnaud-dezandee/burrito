package api

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

func (a *API) GetStackLogsHandler(c echo.Context) error {
	namespace := c.Param("namespace")
	stack := c.Param("stack")
	run := c.Param("run")
	attempt := c.Param("attempt")
	unit := c.QueryParam("unit")
	if namespace == "" || stack == "" || run == "" || attempt == "" {
		return c.String(http.StatusBadRequest, "missing query parameters")
	}
	content, err := a.Datastore.GetStackLogs(namespace, stack, run, attempt, unit)
	if err != nil {
		return c.String(http.StatusInternalServerError, "could not get logs, there's an issue with the storage backend")
	}
	return c.JSON(http.StatusOK, &GetLogsResponse{Results: content})
}

func (a *API) GetStackPlanHandler(c echo.Context) error {
	namespace := c.Param("namespace")
	stack := c.Param("stack")
	run := c.Param("run")
	attempt := c.Param("attempt")
	unit := c.QueryParam("unit")
	format := c.QueryParam("format")
	if format == "" {
		format = "short"
	}
	if namespace == "" || stack == "" || run == "" || attempt == "" {
		return c.String(http.StatusBadRequest, "missing query parameters")
	}
	content, err := a.Datastore.GetStackPlan(namespace, stack, run, attempt, unit, format)
	if err != nil {
		return c.String(http.StatusInternalServerError, "could not get plan, there's an issue with the storage backend")
	}
	return c.Blob(http.StatusOK, "application/octet-stream", content)
}
