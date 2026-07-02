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

## Attributes Reference

In addition to all arguments above, the following attributes are exported:

* `id` - The team unique ID (UUID).

## Import

Teams can be imported using the `organization_name` and `team_name` separated by a colon, e.g.,

```
$ terraform import infradots_team.example infradots:platform-engineering
```
