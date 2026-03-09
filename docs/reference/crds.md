# CRD reference

## `TerragruntStack`

`TerragruntStack` declares an implicit Terragrunt stack rooted at `spec.path`.

Relevant fields:

- `spec.path`: repo-relative root of the implicit stack.
- `spec.branch`: tracked branch.
- `spec.additionalTargetRefs`: optional extra refs that can trigger a full stack run.
- `spec.parallelism`: optional Terragrunt stack parallelism override.
- `spec.terraform`, `spec.opentofu`, `spec.terragrunt`: tool configuration inherited from the repository unless overridden.
- `spec.repository`: linked `TerraformRepository`.
- `spec.remediationStrategy`: accepted for API parity, but `autoApply` is rejected for stacks in V1.
- `spec.overrideRunnerSpec`: runner pod overrides.
- `spec.runHistoryPolicy`: number of historical stack runs to retain.

Status highlights:

- `status.state`: `Idle`, `PlanNeeded`, `ApplyNeeded`, or `Invalid`.
- `status.lastResult`: aggregate summary of the latest stack run.
- `status.lastRun` and `status.latestRuns`: stack run references.
- `status.units`: per-unit summaries keyed by repo-relative unit path.

Each unit entry includes:

- `id` and `path`
- `state`
- `lastAction`
- `lastRun`
- `lastRunAt`
- `lastResult`
- `hasValidPlan`
- `lastPlannedRevision`
- `lastAppliedRevision`
- `isRunning`

Example:

```yaml
apiVersion: config.terraform.padok.cloud/v1alpha1
kind: TerragruntStack
metadata:
  name: platform-prod
  namespace: burrito-project
spec:
  branch: main
  path: live/prod
  parallelism: 4
  repository:
    name: infra-repo
    namespace: burrito-project
  terragrunt:
    enabled: true
  opentofu:
    enabled: true
```

## `TerragruntStackRun`

`TerragruntStackRun` is the execution record for a stack-wide plan or apply.

Relevant fields:

- `spec.action`: `plan` or `apply`.
- `spec.stack.name`
- `spec.stack.namespace`
- `spec.stack.revision`

Status highlights:

- `status.state`
- `status.retries`
- `status.lastRun`
- `status.attempts`
- `status.runnerPod`
- `status.unitResults`

Each `unitResults` entry contains the unit ID, run result, latest relevant revision markers, and execution time.

## Examples

- Manifest example: [`docs/examples/terragrunt-stack.yaml`](../examples/terragrunt-stack.yaml)
- Layout example: [`docs/examples/terragrunt-stack-layout.md`](../examples/terragrunt-stack-layout.md)
