# Integration Resource

The integration resource allows you to create and manage integrations for organizations in Infradots.

## Example Usage

```hcl
resource "infradots_integration" "example" {
  organization_name = "infradots"
  name              = "slack-alerts"
  type              = "SLACK"
  description       = "Posts run notifications to Slack"
}

resource "infradots_integration" "webhook" {
  organization_name = "infradots"
  name              = "deploy-webhook"
  type              = "WEBHOOK"
  api_url           = "https://example.com/hooks/infradots"
  api_key           = "super-secret-token"
}
```

## Argument Reference

The following arguments are supported:

* `organization_name` - (Required) The name of the organization this integration belongs to.
* `name` - (Required) The name of the integration.
* `type` - (Required) The type of integration. Valid values are `WEBHOOK`, `CUSTOM`, and `SLACK`.
* `api_url` - (Optional) The API URL for the integration.
* `api_key` - (Optional) The API key for the integration. This value is write-only and is never returned by the API on read. It is always marked as sensitive in state.
* `description` - (Optional) Description of the integration.

## Attributes Reference

In addition to all arguments above, the following attributes are exported:

* `id` - The integration unique ID (UUID).
* `created_at` - The timestamp when the integration was created.
* `updated_at` - The timestamp when the integration was last updated.

## Import

Integrations can be imported using the `organization_name` and the integration ID (UUID), separated by a colon:

```
$ terraform import infradots_integration.example infradots:a1b2c3d4-e5f6-7890-abcd-ef1234567890
```

Note that `api_key` is write-only and cannot be recovered on import; it will be empty in state afterwards.
