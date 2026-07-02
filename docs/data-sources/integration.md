# Integration Data Source

Use this data source to retrieve details of an existing integration in Infradots.

## Example Usage

```hcl
# Retrieve integration by name and organization name
data "infradots_integration_data" "by_name" {
  organization_name = "example-org"
  name              = "slack-notifications"
}

# Retrieve integration by ID (organization name is still required)
data "infradots_integration_data" "by_id" {
  id                = "3f340e3c-89f1-4321-bcde-eff34567890a"
  organization_name = "example-org"
}
```

## Argument Reference

* `id` - (Optional) The unique ID of the integration to retrieve. Required if `name` is not specified.
* `organization_name` - (Required) The name of the organization this integration belongs to.
* `name` - (Optional) The name of the integration to retrieve. Required if `id` is not specified.

## Attributes Reference

* `id` - The unique ID of the integration.
* `organization_name` - The name of the organization this integration belongs to.
* `name` - The name of the integration.
* `type` - The type of integration (e.g., WEBHOOK, CUSTOM, SLACK).
* `api_url` - The API URL for the integration.
* `description` - Description of the integration.
* `created_at` - The timestamp when the integration was created.
* `updated_at` - The timestamp when the integration was last updated.
