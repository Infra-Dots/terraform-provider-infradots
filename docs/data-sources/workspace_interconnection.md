# Workspace Interconnection Data Source

Use this data source to retrieve details of an existing workspace interconnection in Infradots.

## Example Usage

```hcl
# Retrieve the interconnection details for a workspace
data "infradots_workspace_interconnection_data" "example" {
  organization_name = "example-org"
  workspace_name    = "example-workspace"
}
```

## Argument Reference

* `organization_name` - (Required) The name of the organization.
* `workspace_name` - (Required) The name of the workspace.

## Attributes Reference

* `id` - The unique ID of the interconnection.
* `organization_name` - The name of the organization.
* `workspace_name` - The name of the workspace.
* `to_workspaces` - List of workspace names connected to this workspace.
* `condition` - The condition for triggering connected workspaces.
