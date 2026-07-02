# User Resource

The user resource allows you to add and manage users (members) within organizations in Infradots.

## Example Usage

```hcl
resource "infradots_user" "example" {
  organization_name = "infradots"
  email             = "jane.doe@example.com"
}
```

## Argument Reference

The following arguments are supported:

* `organization_name` - (Required) The name of the organization this user belongs to.
* `email` - (Required) The email address of the user.

## Attributes Reference

In addition to all arguments above, the following attributes are exported:

* `id` - The user unique ID (UUID).
* `last_login` - The timestamp when the user last logged in (RFC3339 format).
* `teams` - A list of team names the user belongs to.
* `permissions` - A list of permissions assigned to the user. Each element is a nested object with the following attributes:
  * `user` - The email of the user the permission applies to.
  * `permission` - The permission codename (e.g., `read_workspaces`, `write_workspaces`).
  * `organization` - The name of the organization the permission is scoped to.
  * `workspace` - The name of the workspace the permission is scoped to. Empty when the permission is organization-level.

## Import

Users can be imported using the `organization_name` and the user `email` separated by a colon, e.g.,

```
$ terraform import infradots_user.example infradots:jane.doe@example.com
```
