# Permission Resource

The permission resource allows you to assign permissions to a user or a team within an organization in Infradots.

## Example Usage

```hcl
resource "infradots_permission" "team_read_workspaces" {
  organization_name = "infradots"
  team_id           = infradots_team.example.id
  permission        = "read_workspaces"
}

resource "infradots_permission" "user_write_workspace" {
  organization_name = "infradots"
  user_email        = "jane.doe@example.com"
  permission        = "write_workspaces"
  workspace_name    = "example-workspace"
}
```

## Argument Reference

The following arguments are supported:

* `organization_name` - (Required) The name of the organization.
* `permission` - (Required) The permission codename (e.g., `read_workspaces`, `write_workspaces`, `read_organizations`, `write_organizations`, `read_teams`, `write_teams`).
* `team_id` - (Optional) The ID of the team to assign the permission to. Mutually exclusive with `user_email`.
* `user_email` - (Optional) The email of the user to assign the permission to. Mutually exclusive with `team_id`.
* `workspace_name` - (Optional) The name of the workspace to scope this permission to. If not set, the permission is organization-level.

## Attributes Reference

In addition to all arguments above, the following attributes are exported:

* `id` - Composite ID for this permission (`organization:permission:user_or_team[:workspace]`).
