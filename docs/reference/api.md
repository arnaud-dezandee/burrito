# API reference

The Burrito server exposes HTTP endpoints for both Terraform layers and Terragrunt stacks.

## Stack endpoints

`GET /api/stacks`

- Returns the list of `TerragruntStack` resources known by Burrito.
- Each result includes stack metadata, aggregate status, latest runs, and unit summaries.

`POST /api/stacks/:namespace/:stack/sync`

- Adds the manual sync annotation to a stack.
- This schedules a fresh stack-wide Terragrunt plan.

`POST /api/stacks/:namespace/:stack/apply`

- Adds the manual apply annotation to a stack.
- V1 always performs a fresh Terragrunt apply and does not reuse a saved plan artifact.

`GET /api/stack-logs/:namespace/:stack/:run/:attempt?unit=<unit-id>`

- Returns stack logs for a specific run attempt.
- Omit `unit` to fetch aggregate stack logs.
- Set `unit` to a repo-relative unit path such as `live/prod/network` to fetch unit-scoped logs.

`GET /api/stack-plans/:namespace/:stack/:run/:attempt?unit=<unit-id>&format=<format>`

- Returns stack plan data for a specific run attempt.
- Supported formats are `short`, `pretty`, and `json`.
- Omit `unit` to fetch the aggregate stack plan summary.

## Notes

- `TerragruntStack` support is limited to implicit stacks.
- Burrito runs the full stack whenever a relevant change hits the stack root or one of its additional trigger paths.
- PR/MR workflows are not supported for stacks in V1.
- `autoApply` is not supported for stacks in V1.
