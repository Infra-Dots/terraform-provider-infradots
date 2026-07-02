# Workspace Interconnection Resource

The workspace interconnection resource lets you orchestrate multiple workspaces by having a source workspace trigger one or more connected workspaces in Infradots.

## Example Usage

```hcl
resource "infradots_workspace_interconnection" "example" {
  organization_name = "infradots"
  workspace_name    = "networking"
  connected_to = [
    "application",
    "database",
  ]
  condition = "full_apply"
}
```

## Argument Reference

The following arguments are supported:

* `organization_name` - (Required) The name of the organization this interconnection belongs to.
* `workspace_name` - (Required) The **name** of the source workspace (the workspace name, not its ID). Runs in this workspace trigger the connected workspaces.
* `connected_to` - (Required) A list of workspace **names** (not IDs) that the source workspace triggers.
* `condition` - (Optional) The condition for triggering the connected workspaces. Valid values are "full_apply" or "always". Defaults to "full_apply".

## Attributes Reference

In addition to all arguments above, the following attributes are exported:

* `id` - The composite ID of the interconnection, in the format `organization_name:workspace_name`.

## Import

Workspace interconnections can be imported using the `organization_name` and the source `workspace_name` (its **name**, not its ID) separated by a colon, e.g.,

```
$ terraform import infradots_workspace_interconnection.example infradots:networking
```
