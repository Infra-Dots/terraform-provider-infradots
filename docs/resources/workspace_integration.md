# Workspace Integration Resource

The workspace integration resource attaches an existing integration to a workspace in Infradots.

## Example Usage

```hcl
resource "infradots_workspace_integration" "example" {
  organization_name = "infradots"
  workspace_name    = "production"
  integration_id    = infradots_integration.example.id
  run_after_stage   = "apply"
  slack_channels    = ["#infra-alerts", "#deploys"]

  slack_env_channels = {
    production = "#prod-alerts"
    staging    = "#staging-alerts"
  }
}
```

## Argument Reference

The following arguments are supported:

* `organization_name` - (Required) The name of the organization.
* `workspace_name` - (Required) The **name** of the workspace to attach the integration to (the workspace name, not its ID).
* `integration_id` - (Required) The ID of the integration to attach. Changing this forces a new resource.
* `run_after_stage` - (Optional) The stage after which the integration runs. Valid values are `init`, `debug`, `details`, `plan`, `apply`, and `all`. Defaults to `apply`.
* `slack_channels` - (Optional) List of Slack channel names.
* `slack_env_channels` - (Optional) Map of environment names to Slack channel names.

## Attributes Reference

In addition to all arguments above, the following attributes are exported:

* `id` - The unique ID of the workspace integration attachment.

## Import

Workspace integrations can be imported using the `organization_name`, the workspace **name**, and the integration ID, separated by colons — `organization_name:workspace_name:integration_id`:

```
$ terraform import infradots_workspace_integration.example infradots:production:a1b2c3d4-e5f6-7890-abcd-ef1234567890
```
