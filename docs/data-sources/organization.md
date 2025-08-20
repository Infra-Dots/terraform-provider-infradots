# Organization Data Source

Use this data source to retrieve details of an existing organization in Infradots.

## Example Usage

```hcl
# Retrieve organization by name
data "infradots_organization_data" "by_name" {
  name = "example-org"
}

# Retrieve organization by ID
data "infradots_organization_data" "by_id" {
  id = "2e240d2c-78e0-4832-abdc-daa33477a238"
}

# Use the data source in other resources
resource "infradots_workspace" "example" {
  organization_name = data.infradots_organization_data.by_name.name
  name              = "example-workspace"
  source            = "https://github.com/example/terraform-config"
  branch            = "main"
  terraform_version = "1.5.0"
}
```

## Argument Reference

* `id` - (Optional) The ID of the organization to retrieve. Either `id` or `name` must be specified.
* `name` - (Optional) The name of the organization to retrieve. Either `id` or `name` must be specified.

## Attributes Reference

* `id` - The ID of the organization.
* `name` - The name of the organization.
* `created_at` - The timestamp when the organization was created.
* `updated_at` - The timestamp when the organization was last updated.
* `execution_mode` - The execution mode for the organization (Remote, Local, etc.).
* `agents_enabled` - Whether agents are enabled for the organization.
* `members` - A list of members in the organization. Each member has the following attributes:
  * `email` - The email address of the member.
* `teams` - A list of teams in the organization. Each team has the following attributes:
  * `name` - The name of the team.
