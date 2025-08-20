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

## Attributes Reference

In addition to all arguments above, the following attributes are exported:

* `id` - The ID of the workspace (UUID).
* `created_at` - The timestamp when the workspace was created (RFC3339 format).
* `updated_at` - The timestamp when the workspace was last updated (RFC3339 format).

## Import

Workspaces can be imported using the `organization_name` and `id` separated by a colon, e.g.,

```
$ terraform import infradots_workspace.example infradots:2e240d2c-78e0-4832-abdc-daa33477a238
