# Workspace Schedule Resource

The workspace schedule resource allows you to manage a cron schedule for a workspace in Infradots.

## Example Usage

```hcl
resource "infradots_workspace_schedule" "example" {
  organization_name = "infradots"
  workspace_name    = "example-workspace"
  type              = "apply"
  crontab           = "0 12 * * *"
}
```

## Argument Reference

The following arguments are supported:

* `organization_name` - (Required) The name of the organization this schedule belongs to.
* `workspace_name` - (Required) The **name** of the workspace this schedule applies to (the workspace name, not its ID).
* `type` - (Required) The schedule type. Valid values are "plan", "apply", "destroy", or "refresh".
* `crontab` - (Required) Cron expression in the format `minute hour day_of_month month_of_year day_of_week` (e.g., `0 12 * * *`).

## Attributes Reference

In addition to all arguments above, the following attributes are exported:

* `id` - The unique ID of the workspace schedule.
* `schedule` - A human-readable representation of the schedule returned by the API.

## Import

Workspace schedules can be imported using the `organization_name`, the workspace **name** (not its ID), and the schedule `id`, separated by colons, e.g.,

```
$ terraform import infradots_workspace_schedule.example infradots:example-workspace:sched-12345
```
