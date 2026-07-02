# Workspace Schedule Data Source

Use this data source to retrieve details of an existing workspace schedule in Infradots.

## Example Usage

```hcl
# Retrieve workspace schedule by type
data "infradots_workspace_schedule_data" "by_type" {
  organization_name = "example-org"
  workspace_name    = "example-workspace"
  type              = "plan"
}

# Retrieve workspace schedule by ID
data "infradots_workspace_schedule_data" "by_id" {
  organization_name = "example-org"
  workspace_name    = "example-workspace"
  id                = "3f340e3c-89f1-4321-bcde-eff34567890a"
}
```

## Argument Reference

* `id` - (Optional) The unique ID of the workspace schedule to retrieve. Required if `type` is not provided.
* `organization_name` - (Required) The name of the organization the workspace belongs to.
* `workspace_name` - (Required) The name of the workspace.
* `type` - (Optional) The type of schedule (e.g., plan, apply). Used as a filter when `id` is not provided. Required if `id` is not provided.

## Attributes Reference

* `id` - The unique ID of the workspace schedule.
* `organization_name` - The name of the organization the workspace belongs to.
* `workspace_name` - The name of the workspace.
* `type` - The type of schedule (e.g., plan, apply).
* `crontab` - The crontab expression for the schedule.
* `schedule` - A human-readable description of the schedule.
