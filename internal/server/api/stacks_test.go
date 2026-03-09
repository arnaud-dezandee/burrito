package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"

	"github.com/labstack/echo/v4"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	configv1alpha1 "github.com/padok-team/burrito/api/v1alpha1"
	"github.com/padok-team/burrito/internal/annotations"
	"github.com/padok-team/burrito/internal/server/api"
	"github.com/padok-team/burrito/internal/server/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("Stacks API", func() {
	var e *echo.Echo

	BeforeEach(func() {
		e = echo.New()
	})

	Describe("StacksHandler", func() {
		It("should return stacks with units and manual operation status", func() {
			stack := &configv1alpha1.TerragruntStack{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "platform",
					Namespace: "default",
					UID:       "stack-uid",
					Annotations: map[string]string{
						annotations.SyncNow:     "true",
						annotations.LastPlanSum: "summary",
					},
				},
				Spec: configv1alpha1.TerragruntStackSpec{
					Path:   "live/prod",
					Branch: "main",
					Repository: configv1alpha1.TerraformLayerRepository{
						Name:      "repo",
						Namespace: "default",
					},
				},
				Status: configv1alpha1.TerragruntStackStatus{
					State:      "Idle",
					LastResult: "network: Plan: 1 to add, 0 to change, 0 to destroy",
					LastRun: configv1alpha1.TerragruntStackRunRef{
						Name:   "platform-plan-1",
						Commit: "abc123",
						Date:   metav1.Now(),
						Action: "plan",
					},
					LatestRuns: []configv1alpha1.TerragruntStackRunRef{{
						Name:   "platform-plan-1",
						Commit: "abc123",
						Date:   metav1.Now(),
						Action: "plan",
					}},
					Units: []configv1alpha1.TerragruntStackUnit{{
						ID:                  "live/prod/network",
						Path:                "live/prod/network",
						State:               "success",
						LastAction:          "plan",
						LastRun:             "platform-plan-1",
						LastRunAt:           metav1.Now(),
						LastResult:          "Plan: 1 to add, 0 to change, 0 to destroy",
						HasValidPlan:        true,
						LastPlannedRevision: "abc123",
						IsRunning:           false,
					}},
				},
			}
			run := &configv1alpha1.TerragruntStackRun{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "platform-plan-1",
					Namespace:         "default",
					CreationTimestamp: metav1.Now(),
				},
				Spec: configv1alpha1.TerragruntStackRunSpec{
					Action: "plan",
					Stack: configv1alpha1.TerragruntStackRunStack{
						Name:      "platform",
						Namespace: "default",
						Revision:  "abc123",
					},
				},
				Status: configv1alpha1.TerragruntStackRunStatus{
					State: "Succeeded",
				},
			}

			fakeClient := fake.NewClientBuilder().
				WithScheme(newScheme()).
				WithObjects(stack, run).
				Build()

			a := &api.API{Client: fakeClient}

			req := httptest.NewRequest(http.MethodGet, "/api/stacks", nil)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			err := a.StacksHandler(c)
			Expect(err).NotTo(HaveOccurred())
			Expect(rec.Code).To(Equal(http.StatusOK))

			var resp struct {
				Results []struct {
					Name             string                 `json:"name"`
					Repository       string                 `json:"repository"`
					ManualSyncStatus utils.ManualSyncStatus `json:"manualSyncStatus"`
					Units            []struct {
						ID           string `json:"id"`
						HasValidPlan bool   `json:"hasValidPlan"`
					} `json:"units"`
				} `json:"results"`
			}
			Expect(json.Unmarshal(rec.Body.Bytes(), &resp)).NotTo(HaveOccurred())
			Expect(resp.Results).To(HaveLen(1))
			Expect(resp.Results[0].Name).To(Equal("platform"))
			Expect(resp.Results[0].Repository).To(Equal("default/repo"))
			Expect(resp.Results[0].ManualSyncStatus).To(Equal(utils.ManualSyncAnnotated))
			Expect(resp.Results[0].Units).To(HaveLen(1))
			Expect(resp.Results[0].Units[0].ID).To(Equal("live/prod/network"))
			Expect(resp.Results[0].Units[0].HasValidPlan).To(BeTrue())
		})
	})
})
