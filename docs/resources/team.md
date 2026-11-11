# Team Resource

The team resource allows you to create and manage teams within organizations in Infradots.

## Example Usage

```hcl
resource "infradots_team" "example" {
  organization_name = "infradots"
  name              = "platform-engineering"
  members = [
    "jane.doe@example.com",
    "john.smith@example.com",
  ]
}
```

## Argument Reference

The following arguments are supported:

* `organization_name` - (Required) The name of the organization this team belongs to.
* `name` - (Required) The name of the team.
* `members` - (Optional) A list of member email addresses in the team. If omitted, the team is created with no members.
* `organization_admin` - (Optional) Whether the team administers the organization; its members and team tokens then
  hold every permission, billing included. Changing it needs organization write access, and an organization always
  keeps at least one administrator user or team. Omit it to leave the current value alone. Switching it off also
  clears the team's organization-level permissions; grant the ones it keeps with `infradots_permission`.

## Attributes Reference

In addition to all arguments above, the following attributes are exported:

* `id` - The team unique ID (UUID).

## Import

Teams can be imported using the `organization_name` and `team_name` separated by a colon, e.g.,

```
$ terraform import infradots_team.example infradots:platform-engineering
```
