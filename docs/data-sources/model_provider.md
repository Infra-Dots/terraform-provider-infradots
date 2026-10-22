# Model Provider Data Source

Use this data source to retrieve details of an existing model provider in Infradots.

## Example Usage

```hcl
# Retrieve model provider by name and organization name
data "infradots_model_provider_data" "by_name" {
  organization_name = "example-org"
  name              = "our-bedrock"
}

# Retrieve model provider by ID (organization name is still required)
data "infradots_model_provider_data" "by_id" {
  id                = "3f340e3c-89f1-4321-bcde-eff34567890a"
  organization_name = "example-org"
}
```

## Argument Reference

* `id` - (Optional) The unique ID of the model provider to retrieve. Required if `name` is not specified.
* `organization_name` - (Required) The name of the organization.
* `name` - (Optional) The name of the model provider to retrieve. Required if `id` is not specified.

## Attributes Reference

* `id` - The unique ID of the model provider.
* `organization_name` - The name of the organization.
* `name` - The name of the model provider.
* `provider_type` - Where the organization's model calls go: `anthropic`, `bedrock`, `vertex` or
  `foundry`.
* `config` - The provider's non-secret settings (a region, a project, an endpoint). The secret ones
  are never returned by the API, so they are not available here.
* `model_map` - Each model InfraDots asks for, mapped to this provider's own id.
* `enabled` - Whether this provider may serve runs.
* `verified_at` - When this provider last answered a verification call, if it ever has.
* `verification_error` - Why the last verification failed, if it did.
* `description` - A description of the model provider.
* `created_at` - The timestamp when the model provider was created.
* `updated_at` - The timestamp when the model provider was last updated.
