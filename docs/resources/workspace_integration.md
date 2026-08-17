# Workspace Integration Resource

The workspace integration resource attaches an existing integration to a workspace in Infradots, and
chooses which events notify it.

## Example Usage

```hcl
resource "infradots_workspace_integration" "example" {
  organization_name = "infradots"
  workspace_name    = "production"
  integration_id    = infradots_integration.example.id

  events = [
    "run.waiting_approval",
    "run.applied",
    "run.failed",
    "agent.review_completed",
  ]

  slack_channels = ["#infra-alerts", "#deploys"]

  slack_env_channels = {
    production = "#prod-alerts"
    staging    = "#staging-alerts"
  }
}
```

## Events

Integrations are triggered by named events. The authoritative list is served by the API — fetch it
with `GET /api/organizations/{organization_name}/integrations/event-types/` — and currently covers:

| Event | Meaning |
|---|---|
| `run.plan_completed` | A plan finished successfully and is ready to review. |
| `run.waiting_approval` | A run produced a plan and is blocked on manual approval. |
| `run.approved` | A pending run was approved and is queued to apply. |
| `run.rejected` | A pending run was discarded instead of applied. |
| `run.applied` | A run applied its changes to infrastructure. |
| `run.failed` | A run failed during plan or apply. |
| `run.cancelled` | A queued or running job was cancelled. |
| `agent.review_completed` | The review agent finished analysing a change. |
| `agent.review_failed` | The review agent could not complete its analysis. |
| `agent.implement_completed` | The implementation agent finished writing infrastructure code. |
| `agent.implement_failed` | The implementation agent could not complete its work. |
| `agent.pr_opened` | An agent opened a pull request with its changes. |
| `agent.risk_assessed` | The risk agent classified the risk of a change. |
| `agent.drift_detected` | The drift agent found infrastructure that no longer matches code. |

Omitting `events` uses the integration's organization-level defaults (the `run.*` events). Setting it
to `[]` mutes the integration without detaching it.

## Argument Reference

The following arguments are supported:

* `organization_name` - (Required) The name of the organization.
* `workspace_name` - (Required) The **name** of the workspace to attach the integration to (the workspace name, not its ID).
* `integration_id` - (Required) The ID of the integration to attach. Changing this forces a new resource.
* `events` - (Optional) List of events that notify this integration. Changing this forces a new resource, because a subscription can only be set when the integration is attached.
* `slack_channels` - (Optional) List of Slack channel names.
* `slack_env_channels` - (Optional) Map of environment names to Slack channel names.
* `run_after_stage` - (Deprecated) No longer used. Integrations are triggered by `events` now; this attribute is accepted but ignored, and will be removed in the next minor release. It never actually filtered anything — every attached integration received every notification regardless of its value.

## Attributes Reference

In addition to all arguments above, the following attributes are exported:

* `id` - The unique ID of the workspace integration attachment.

## Import

Workspace integrations can be imported using the `organization_name`, the workspace **name**, and the integration ID, separated by colons — `organization_name:workspace_name:integration_id`:

```
$ terraform import infradots_workspace_integration.example infradots:production:a1b2c3d4-e5f6-7890-abcd-ef1234567890
```
