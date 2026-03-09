// nolint
package api_test

import (
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Datastore Stack API", func() {
	Describe("Stack Logs", func() {
		It("should return stack logs with a 200 OK", func() {
			Expect(API.Storage.PutStackLogs("default", "stack1", "run1", "0", "", []byte("stack-log"))).To(Succeed())

			context := getContext(http.MethodGet, "/stack/logs", map[string]string{
				"namespace": "default",
				"stack":     "stack1",
				"run":       "run1",
				"attempt":   "0",
			}, nil)
			err := API.GetStackLogsHandler(context)
			Expect(err).NotTo(HaveOccurred())
			Expect(context.Response().Status).To(Equal(http.StatusOK))
		})

		It("should return 404 when stack logs do not exist", func() {
			context := getContext(http.MethodGet, "/stack/logs", map[string]string{
				"namespace": "default",
				"stack":     "missing-stack",
				"run":       "missing-run",
				"attempt":   "0",
			}, nil)
			err := API.GetStackLogsHandler(context)
			Expect(err).NotTo(HaveOccurred())
			Expect(context.Response().Status).To(Equal(http.StatusNotFound))
		})
	})

	Describe("Stack Plans", func() {
		It("should return stack plans with a 200 OK", func() {
			Expect(API.Storage.PutStackPlan("default", "stack1", "run1", "0", "live/prod/network", "json", []byte("{}"))).To(Succeed())

			context := getContext(http.MethodGet, "/stack/plans", map[string]string{
				"namespace": "default",
				"stack":     "stack1",
				"run":       "run1",
				"attempt":   "0",
				"unit":      "live/prod/network",
				"format":    "json",
			}, nil)
			err := API.GetStackPlanHandler(context)
			Expect(err).NotTo(HaveOccurred())
			Expect(context.Response().Status).To(Equal(http.StatusOK))
		})

		It("should return 404 when stack plans do not exist", func() {
			context := getContext(http.MethodGet, "/stack/plans", map[string]string{
				"namespace": "default",
				"stack":     "missing-stack",
				"run":       "missing-run",
				"attempt":   "0",
				"unit":      "live/prod/network",
				"format":    "json",
			}, nil)
			err := API.GetStackPlanHandler(context)
			Expect(err).NotTo(HaveOccurred())
			Expect(context.Response().Status).To(Equal(http.StatusNotFound))
		})
	})
})
