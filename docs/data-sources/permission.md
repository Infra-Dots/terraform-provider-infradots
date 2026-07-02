# Permission Data Source

Use this data source to retrieve permission mappings of an existing organization in Infradots.

## Example Usage

```hcl
# Retrieve all permission mappings for an organization
data "infradots_permission_data" "all" {
  organization_name = "example-org"
}

# Retrieve permission mappings filtered by workspace name and permission name
data "infradots_permission_data" "filtered" {
  organization_name = "example-org"
  workspace_name    = "example-workspace"
  permission_name   = "admin"
}
```

## Argument Reference

* `organization_name` - (Required) The name of the organization to retrieve permission mappings for.
* `permission_name` - (Optional) Filter by permission name.
* `team_id` - (Optional) Filter by team ID.
* `user_email` - (Optional) Filter by user email.
* `workspace_name` - (Optional) Filter by workspace name.

## Attributes Reference

* `permissions` - List of matching permission mappings. Each element is a nested object with the following attributes:
  * `id` - The unique ID of the permission mapping.
  * `permission_name` - The name of the permission.
  * `team_id` - The ID of the team the permission is assigned to.
  * `user_email` - The email address of the user the permission is assigned to.
  * `workspace_name` - The name of the workspace the permission applies to.
