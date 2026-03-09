package runner

import (
	"context"
	"crypto/sha256"
	b64 "encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	tfjson "github.com/hashicorp/terraform-json"
	configv1alpha1 "github.com/padok-team/burrito/api/v1alpha1"
	"github.com/padok-team/burrito/internal/annotations"
	tgtool "github.com/padok-team/burrito/internal/runner/tools/terragrunt"
	runnerutils "github.com/padok-team/burrito/internal/utils/runner"
	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const PlanArtifact string = "/tmp/plan.out"

type terragruntRunReportEntry struct {
	Started string   `json:"Started"`
	Ended   string   `json:"Ended"`
	Reason  string   `json:"Reason"`
	Cause   string   `json:"Cause"`
	Name    string   `json:"Name"`
	Result  string   `json:"Result"`
	Ref     string   `json:"Ref"`
	Cmd     string   `json:"Cmd"`
	Args    []string `json:"Args"`
}

func (r *Runner) ExecAction() error {
	if r.isStackTarget() {
		return r.execStackAction()
	}
	return r.execLayerAction()
}

func (r *Runner) execLayerAction() error {
	ann := map[string]string{}

	switch r.config.Runner.Action {
	case "plan":
		sum, err := r.execPlan()
		if err != nil {
			return err
		}
		ann[annotations.LastPlanDate] = time.Now().Format(time.UnixDate)
		ann[annotations.LastPlanRun] = fmt.Sprintf("%s/%s", r.Run.Name, strconv.Itoa(r.Run.Status.Retries))
		ann[annotations.LastPlanSum] = sum
		ann[annotations.LastPlanCommit] = r.Run.Spec.Layer.Revision
	case "apply":
		sum, err := r.execApply()
		if err != nil {
			return err
		}
		ann[annotations.LastApplyDate] = time.Now().Format(time.UnixDate)
		ann[annotations.LastApplySum] = sum
		ann[annotations.LastApplyCommit] = r.Run.Spec.Layer.Revision
	default:
		return errors.New("unrecognized runner action, if this is happening there might be a version mismatch between the controller and runner")
	}

	if err := annotations.Add(context.TODO(), r.Client, r.Layer, ann); err != nil {
		log.Errorf("could not update TerraformLayer annotations: %s", err)
		return err
	}
	return nil
}

func (r *Runner) execStackAction() error {
	ann := map[string]string{}
	switch r.config.Runner.Action {
	case "plan":
		sum, units, err := r.execStackPlan()
		if err != nil {
			return err
		}
		ann[annotations.LastPlanDate] = time.Now().Format(time.UnixDate)
		ann[annotations.LastPlanRun] = fmt.Sprintf("%s/%s", r.StackRun.Name, strconv.Itoa(r.StackRun.Status.Retries))
		ann[annotations.LastPlanSum] = sum
		ann[annotations.LastPlanCommit] = r.StackRun.Spec.Stack.Revision
		r.StackRun.Status.UnitResults = units
	case "apply":
		sum, units, err := r.execStackApply()
		if err != nil {
			return err
		}
		ann[annotations.LastApplyDate] = time.Now().Format(time.UnixDate)
		ann[annotations.LastApplySum] = sum
		ann[annotations.LastApplyCommit] = r.StackRun.Spec.Stack.Revision
		r.StackRun.Status.UnitResults = units
	default:
		return errors.New("unrecognized runner action, if this is happening there might be a version mismatch between the controller and runner")
	}
	if err := annotations.Add(context.TODO(), r.Client, r.Stack, ann); err != nil {
		log.Errorf("could not update TerragruntStack annotations: %s", err)
		return err
	}
	if err := r.Client.Status().Update(context.TODO(), r.StackRun); err != nil {
		log.Errorf("could not update TerragruntStackRun status: %s", err)
		return err
	}
	return nil
}

func (r *Runner) ExecInit() error {
	log.Infof("launching %s init in %s", r.exec.TenvName(), r.workingDir)
	if r.exec == nil {
		return errors.New("terraform or terragrunt binary not installed")
	}
	if err := r.exec.Init(r.workingDir); err != nil {
		log.Errorf("error executing %s init: %s", r.exec.TenvName(), err)
		return err
	}
	return nil
}

func (r *Runner) execPlan() (string, error) {
	log.Infof("running %s plan", r.exec.TenvName())
	if r.exec == nil {
		return "", errors.New("terraform or terragrunt binary not installed")
	}
	if err := r.exec.Plan(PlanArtifact); err != nil {
		log.Errorf("error executing %s plan: %s", r.exec.TenvName(), err)
		return "", err
	}
	planJsonBytes, err := r.exec.Show(PlanArtifact, "json")
	if err != nil {
		return "", err
	}
	prettyPlan, err := r.exec.Show(PlanArtifact, "pretty")
	if err != nil {
		return "", err
	}
	_ = r.Datastore.PutPlan(r.Layer.Namespace, r.Layer.Name, r.Run.Name, strconv.Itoa(r.Run.Status.Retries), "pretty", prettyPlan)
	plan := &tfjson.Plan{}
	if err := json.Unmarshal(planJsonBytes, plan); err != nil {
		return "", err
	}
	_, shortDiff := runnerutils.GetDiff(plan)
	_ = r.Datastore.PutPlan(r.Layer.Namespace, r.Layer.Name, r.Run.Name, strconv.Itoa(r.Run.Status.Retries), "json", planJsonBytes)
	_ = r.Datastore.PutPlan(r.Layer.Namespace, r.Layer.Name, r.Run.Name, strconv.Itoa(r.Run.Status.Retries), "short", []byte(shortDiff))
	planBin, err := os.ReadFile(PlanArtifact)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(planBin)
	if err := r.Datastore.PutPlan(r.Layer.Namespace, r.Layer.Name, r.Run.Name, strconv.Itoa(r.Run.Status.Retries), "bin", planBin); err != nil {
		return "", err
	}
	return b64.StdEncoding.EncodeToString(sum[:]), nil
}

func (r *Runner) execApply() (string, error) {
	log.Infof("starting %s apply", r.exec.TenvName())
	if r.exec == nil {
		return "", fmt.Errorf("%s binary not installed", r.exec.TenvName())
	}
	plan, err := r.Datastore.GetPlan(r.Layer.Namespace, r.Layer.Name, r.Run.Spec.Artifact.Run, r.Run.Spec.Artifact.Attempt, "bin")
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(plan)
	if err := os.WriteFile(PlanArtifact, plan, 0644); err != nil {
		return "", err
	}
	if configv1alpha1.GetApplyWithoutPlanArtifactEnabled(r.Repository, r.Layer) {
		err = r.exec.Apply("")
	} else {
		err = r.exec.Apply(PlanArtifact)
	}
	if err != nil {
		return "", err
	}
	_ = r.Datastore.PutPlan(r.Layer.Namespace, r.Layer.Name, r.Run.Name, strconv.Itoa(r.Run.Status.Retries), "short", []byte("Apply Successful"))
	return b64.StdEncoding.EncodeToString(sum[:]), nil
}

func (r *Runner) execStackPlan() (string, []configv1alpha1.TerragruntStackUnitResult, error) {
	tg, ok := r.exec.(*tgtool.Terragrunt)
	if !ok {
		return "", nil, errors.New("terragrunt stack runner requires the terragrunt tool")
	}
	outDir := filepath.Join(os.TempDir(), "terragrunt-stack-plan")
	jsonOutDir := filepath.Join(os.TempDir(), "terragrunt-stack-json")
	reportFile := filepath.Join(os.TempDir(), "terragrunt-stack-report.json")
	_ = os.RemoveAll(outDir)
	_ = os.RemoveAll(jsonOutDir)
	if err := tg.RunAllPlan(outDir, jsonOutDir, reportFile, r.Stack.Spec.Parallelism); err != nil {
		return "", nil, err
	}
	report, err := readRunReport(reportFile)
	if err != nil {
		return "", nil, err
	}

	attempt := strconv.Itoa(r.StackRun.Status.Retries)
	unitResults := make([]configv1alpha1.TerragruntStackUnitResult, 0, len(report))
	aggregateShorts := []string{}
	for _, entry := range report {
		unitPath := filepath.Clean(entry.Name)
		prettyPlan := []byte(entry.Result)
		jsonPlanPath := filepath.Join(jsonOutDir, unitPath, "tfplan.json")
		planPath := filepath.Join(outDir, unitPath, "tfplan.tfplan")
		hasValidPlan := false
		short := entry.Result
		jsonPlan := []byte{}

		if b, err := os.ReadFile(jsonPlanPath); err == nil {
			jsonPlan = b
			hasValidPlan = true
			plan := &tfjson.Plan{}
			if err := json.Unmarshal(jsonPlan, plan); err == nil {
				_, short = runnerutils.GetDiff(plan)
			}
			_ = r.Datastore.PutStackPlan(r.Stack.Namespace, r.Stack.Name, r.StackRun.Name, attempt, unitPath, "json", jsonPlan)
		}
		if b, err := tg.ShowPlanFile(planPath, "pretty"); err == nil {
			prettyPlan = b
			_ = r.Datastore.PutStackPlan(r.Stack.Namespace, r.Stack.Name, r.StackRun.Name, attempt, unitPath, "pretty", prettyPlan)
		}
		_ = r.Datastore.PutStackPlan(r.Stack.Namespace, r.Stack.Name, r.StackRun.Name, attempt, unitPath, "short", []byte(short))

		result := configv1alpha1.TerragruntStackUnitResult{
			Run:                 r.StackRun.Name,
			ID:                  unitPath,
			Path:                unitPath,
			State:               entry.Result,
			Action:              "plan",
			Result:              short,
			HasValidPlan:        hasValidPlan,
			LastPlannedRevision: r.StackRun.Spec.Stack.Revision,
			RunAt:               metav1Now(),
		}
		unitResults = append(unitResults, result)
		aggregateShorts = append(aggregateShorts, fmt.Sprintf("%s: %s", unitPath, short))
	}
	aggregateShort := strings.Join(aggregateShorts, "\n")
	_ = r.Datastore.PutStackPlan(r.Stack.Namespace, r.Stack.Name, r.StackRun.Name, attempt, "", "short", []byte(aggregateShort))
	_ = r.Datastore.PutStackPlan(r.Stack.Namespace, r.Stack.Name, r.StackRun.Name, attempt, "", "pretty", []byte(aggregateShort))
	sum := sha256.Sum256([]byte(aggregateShort))
	return b64.StdEncoding.EncodeToString(sum[:]), unitResults, nil
}

func (r *Runner) execStackApply() (string, []configv1alpha1.TerragruntStackUnitResult, error) {
	tg, ok := r.exec.(*tgtool.Terragrunt)
	if !ok {
		return "", nil, errors.New("terragrunt stack runner requires the terragrunt tool")
	}
	reportFile := filepath.Join(os.TempDir(), "terragrunt-stack-report.json")
	if err := tg.RunAllApply(reportFile, r.Stack.Spec.Parallelism); err != nil {
		return "", nil, err
	}
	report, err := readRunReport(reportFile)
	if err != nil {
		return "", nil, err
	}
	attempt := strconv.Itoa(r.StackRun.Status.Retries)
	unitResults := make([]configv1alpha1.TerragruntStackUnitResult, 0, len(report))
	aggregateShorts := []string{}
	for _, entry := range report {
		unitPath := filepath.Clean(entry.Name)
		result := entry.Result
		if entry.Cause != "" {
			result = fmt.Sprintf("%s: %s", entry.Result, entry.Cause)
		}
		unitResult := configv1alpha1.TerragruntStackUnitResult{
			Run:                 r.StackRun.Name,
			ID:                  unitPath,
			Path:                unitPath,
			State:               entry.Result,
			Action:              "apply",
			Result:              result,
			LastAppliedRevision: r.StackRun.Spec.Stack.Revision,
			RunAt:               metav1Now(),
		}
		unitResults = append(unitResults, unitResult)
		aggregateShorts = append(aggregateShorts, fmt.Sprintf("%s: %s", unitPath, result))
		_ = r.Datastore.PutStackPlan(r.Stack.Namespace, r.Stack.Name, r.StackRun.Name, attempt, unitPath, "short", []byte(result))
	}
	aggregateShort := strings.Join(aggregateShorts, "\n")
	_ = r.Datastore.PutStackPlan(r.Stack.Namespace, r.Stack.Name, r.StackRun.Name, attempt, "", "short", []byte(aggregateShort))
	sum := sha256.Sum256([]byte(aggregateShort))
	return b64.StdEncoding.EncodeToString(sum[:]), unitResults, nil
}

func readRunReport(path string) ([]terragruntRunReportEntry, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	report := []terragruntRunReportEntry{}
	if err := json.Unmarshal(content, &report); err != nil {
		return nil, err
	}
	return report, nil
}

func metav1Now() metav1.Time {
	return metav1.NewTime(time.Now())
}
