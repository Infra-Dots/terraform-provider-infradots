# Model Provider Resource

The model provider resource allows you to create and manage AI model providers within organizations in Infradots.

## Example Usage

```hcl
resource "infradots_model_provider" "example" {
  organization_name = "infradots"
  name              = "primary-anthropic"
  provider_type     = "anthropic"
  api_key           = "sk-ant-..."
  description        = "Anthropic provider for agent skills"
}
```

## Argument Reference

The following arguments are supported:

* `organization_name` - (Required) The name of the organization this model provider belongs to.
* `name` - (Required) The name of the model provider.
* `provider_type` - (Required) The provider type. Valid values are `openai`, `anthropic`, `google`, `azure_openai`, `cohere`, and `llama`.
* `api_key` - (Required) The API key for the model provider. This value is write-only and is never read back from the API. It is always marked as sensitive in state.
* `description` - (Optional) A description of the model provider.

## Attributes Reference

In addition to all arguments above, the following attributes are exported:

* `id` - The unique ID of the model provider.
* `created_at` - The timestamp when the model provider was created.
* `updated_at` - The timestamp when the model provider was last updated.

## Import

Model providers can be imported using the `organization_name` and the model provider ID, separated by a colon:

```
$ terraform import infradots_model_provider.example infradots:a1b2c3d4-e5f6-7890-abcd-ef1234567890
```

Note that `api_key` is write-only and cannot be recovered on import; it will be empty in state afterwards.
