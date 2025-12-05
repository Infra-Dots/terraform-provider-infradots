# Workspace Data Source

Use this data source to retrieve details of an existing workspace in Infradots.

## Example Usage

```hcl
# Retrieve workspace by name and organization name
data "infradots_workspace_data" "by_name" {
  organization_name = "example-org"
  name              = "example-workspace"
}

# Retrieve workspace by ID (organization name is still required)
data "infradots_workspace_data" "by_id" {
  id                = "3f340e3c-89f1-4321-bcde-eff34567890a"
  organization_name = "example-org"
}

# Use the data source to reference an existing workspace
resource "infradots_variable" "example" {
  organization_name = data.infradots_workspace_data.by_name.organization_name
  key               = "environment"
  value             = "production"
  description       = "Environment for ${data.infradots_workspace_data.by_name.name} workspace"
  category          = "terraform"
}
```

## Argument Reference

* `id` - (Optional) The ID of the workspace to retrieve. If specified, `organization_name` is also required.
* `organization_name` - (Required) The name of the organization the workspace belongs to.
* `name` - (Optional) The name of the workspace to retrieve. Required if `id` is not specified.

## Attributes Reference

* `id` - The ID of the workspace.
* `organization_name` - The name of the organization the workspace belongs to.
* `name` - The name of the workspace.
* `description` - A description of the workspace.
* `source` - Source repository URL or path.
* `branch` - Git branch used (if applicable).
* `terraform_version` - Terraform version used.
* `created_at` - The timestamp when the workspace was created.
* `updated_at` - The timestamp when the workspace was last updated.
* `vcs` - VCS connection details associated with this workspace. This is a nested object with the following attributes:
  * `id` - The VCS unique ID (UUID).
  * `name` - The name of the VCS connection.
  * `vcs_type` - The type of VCS (e.g., github, gitlab, bitbucket).
  * `url` - The URL of the VCS instance.
  * `description` - A description of the VCS connection.
  * `created_at` - The timestamp when the VCS was created.
  * `updated_at` - The timestamp when the VCS was last updated.
