# Workspace Resource

The workspace resource allows you to create and manage workspaces within organizations in Infradots.

## Example Usage

```hcl
resource "infradots_workspace" "example" {
  organization_name = "infradots"
  name              = "example-workspace"
  description       = "Example workspace for terraform configurations"
  source            = "https://github.com/example/terraform-config"
  branch            = "main"
  terraform_version = "1.5.0"

  # Runs in the prod/ working directory, and is also triggered by changes to
  # shared modules and root variable files outside that directory.
  folder = "prod"
  trigger_patterns = [
    { pattern = "modules/vpc/.*" },
    { pattern = "shared/.*\\.tfvars$", enabled = false },
  ]
}
```

## Argument Reference

The following arguments are supported:

* `organization_name` - (Required) The name of the organization this workspace belongs to.
* `name` - (Required) The name of the workspace.
* `description` - (Optional) A short description of the workspace.
* `source` - (Required) Source repository URL or path.
* `branch` - (Required) Git branch to use.
* `terraform_version` - (Required) Terraform version to use for this workspace.
* `vcs_id` - (Optional) ID of a VCS Provider in infradots to connect to the workspace.
* `folder` - (Optional) The working directory (subfolder) within the source repository where commands run. Defaults to `/`. Changes under this folder trigger a run.
* `trigger_patterns` - (Optional) A list of regex patterns matched against the changed file paths of a VCS push or pull request. A changed file triggers a run for this workspace if any enabled pattern matches, **in addition to** changes under `folder` (the two rules are OR'd) — use it to also run on shared modules or root variable files outside the working directory. Omitting the argument keeps any existing patterns; set it to `[]` to clear them. Each element supports:
  * `pattern` - (Required) A regular expression matched against repo-relative changed file paths (e.g. `modules/vpc/.*`). Anchor with `^`/`$` as needed. Validated to compile server-side; an invalid pattern is rejected.
  * `enabled` - (Optional) Whether the pattern is active. Defaults to `true`.

## Attributes Reference

In addition to all arguments above, the following attributes are exported:

* `id` - The ID of the workspace (UUID).
* `created_at` - The timestamp when the workspace was created (RFC3339 format).
* `updated_at` - The timestamp when the workspace was last updated (RFC3339 format).
* `vcs` - VCS connection details associated with this workspace. This is a nested object with the following attributes:
  * `id` - The VCS unique ID (UUID).
  * `name` - The name of the VCS connection.
  * `vcs_type` - The type of VCS (e.g., github, gitlab, bitbucket).
  * `url` - The URL of the VCS instance.
  * `description` - A description of the VCS connection.
  * `created_at` - The timestamp when the VCS was created.
  * `updated_at` - The timestamp when the VCS was last updated.

## Import

Workspaces can be imported using the `organization_name` and `workspace_name` separated by a colon, e.g.,

```
$ terraform import infradots_workspace.example infradots:example-workspace
