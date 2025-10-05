# VCS Data Source

Use this data source to retrieve details of an existing VCS connection in Infradots.

## Example Usage

```hcl
# Retrieve VCS connection by name and organization name
data "infradots_vcs_data" "by_name" {
  organization_name = "example-org"
  name              = "github-connection"
}

# Retrieve VCS connection by ID (organization name is still required)
data "infradots_vcs_data" "by_id" {
  id                = "5f560f5e-0bf3-6543-defg-g1156789012c"
  organization_name = "example-org"
}

# Use the data source to reference an existing VCS connection
resource "infradots_workspace" "example" {
  organization_name = data.infradots_vcs_data.by_name.organization_name
  name              = "example-workspace"
  source            = "https://github.com/example/terraform-config"
  branch            = "main"
  terraform_version = "1.5.0"
}
```

## Argument Reference

* `id` - (Optional) The ID of the VCS connection to retrieve. If specified, `organization_name` is also required.
* `organization_name` - (Required) The name of the organization the VCS connection belongs to.
* `name` - (Optional) The name of the VCS connection to retrieve. Required if `id` is not specified.

## Attributes Reference

* `id` - The ID of the VCS connection.
* `organization_name` - The name of the organization the VCS connection belongs to.
* `name` - The name of the VCS connection.
* `vcs_type` - The type of VCS (e.g., github, gitlab, bitbucket).
* `url` - The URL of the VCS instance.
* `token` - The access token for the VCS.
* `description` - A description of the VCS connection.
* `created_at` - The timestamp when the VCS connection was created.
* `updated_at` - The timestamp when the VCS connection was last updated.
