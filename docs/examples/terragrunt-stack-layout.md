# Implicit stack layout example

An implicit stack is defined by pointing Burrito at a Terragrunt root directory. Burrito then delegates ordering and concurrency to Terragrunt for every unit under that root.

Example repository layout:

```text
live/
  prod/
    root.hcl
    network/
      terragrunt.hcl
    security/
      terragrunt.hcl
    app/
      terragrunt.hcl
modules/
  network/
  security/
  app/
```

Example stack:

```yaml
apiVersion: config.terraform.padok.cloud/v1alpha1
kind: TerragruntStack
metadata:
  name: platform-prod
  namespace: burrito-project
spec:
  branch: main
  path: live/prod
  repository:
    name: infra-repo
    namespace: burrito-project
```

Behavior in V1:

- Any relevant change under `live/prod` triggers a full stack run.
- Additional trigger paths can be used for shared modules outside the stack root.
- Burrito does not auto-discover stacks from repositories.
- PR/MR workflows are not supported for stacks.
- `autoApply` is not supported for stacks.
