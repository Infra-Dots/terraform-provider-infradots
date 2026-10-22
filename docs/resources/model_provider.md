# Model Provider Resource

An account of your own that your organization's agents send their model calls to — "bring your own
model". The same Claude models InfraDots runs, reached through your Anthropic, AWS, Google or Azure
account, so prompts, tools and reviews are unchanged and only the model bill moves.

Creating a provider does not switch anything over by itself. Point the organization at it with
[`infradots_organization_model_provider`](organization_model_provider.md).

## Example Usage

Your own Anthropic account:

```hcl
resource "infradots_model_provider" "anthropic" {
  organization_name = "infradots"
  name              = "our-anthropic"
  provider_type     = "anthropic"
  api_key           = var.anthropic_api_key
  description       = "Our own Anthropic account"
}
```

Claude on your AWS account, through Bedrock:

```hcl
resource "infradots_model_provider" "bedrock" {
  organization_name = "infradots"
  name              = "our-bedrock"
  provider_type     = "bedrock"

  settings = {
    aws_region     = "eu-west-1"
    aws_access_key = var.aws_access_key
    aws_secret_key = var.aws_secret_key
  }

  # Required for bedrock, vertex and foundry: a run asking for a model that is not mapped here stops
  # before it starts, saying which one to add.
  model_map = {
    "claude-opus-5"   = "eu.anthropic.claude-opus-5-v1:0"
    "claude-sonnet-5" = "eu.anthropic.claude-sonnet-5-v1:0"
  }
}
```

## Argument Reference

* `organization_name` - (Required) The organization this model provider belongs to.
* `name` - (Required) The name of the model provider. Unique within the organization.
* `provider_type` - (Required) Where the calls go. One of `anthropic`, `bedrock`, `vertex`,
  `foundry`.
* `settings` - (Optional) The provider's client settings, by their own names. Write-only as a whole,
  because InfraDots cannot return the secret half; the non-secret half reads back under `config`.
  Per provider:

  | `provider_type` | settings |
  |---|---|
  | `anthropic` | `api_key`\*, `base_url` |
  | `bedrock` | `aws_region`, `aws_access_key`\*, `aws_secret_key`\*, `aws_session_token` |
  | `vertex` | `project_id`, `region`, `access_token`\* |
  | `foundry` | `resource`, `api_key`\*, `base_url` |

  The starred ones are required: they are what says *whose* account this is. Without them the
  underlying SDK falls back to the environment of the process making the call — InfraDots' own — so
  a provider without them is rejected rather than quietly billing us.

* `api_key` - (Optional) Shorthand for `settings.api_key`, for the providers that take one. Also
  write-only.
* `model_map` - (Optional) Each model InfraDots asks for, mapped to this provider's own id.
  Required in practice for `bedrock`, `vertex` and `foundry`, whose model ids are not the platform's
  names. Optional for `anthropic`, where it pins a name to a specific snapshot.
* `enabled` - (Optional) Whether this provider may serve runs. Defaults to `true`.
* `description` - (Optional) A description of the model provider.

## Attributes Reference

* `id` - The unique ID of the model provider.
* `config` - The non-secret half of `settings`, as InfraDots holds it. A region changed outside
  Terraform shows up here as drift; a changed secret cannot, since it is never returned.
* `verified_at` - When this provider last answered a verification call, if it ever has. Verification
  is a single cheap call, available in the UI and the API
  (`POST /api/organizations/<org>/model-providers/<id>/verify/`); it is the quickest way to find a
  wrong key, which would otherwise surface six minutes into a review.
* `verification_error` - Why the last verification failed, if it did.
* `created_at`, `updated_at` - Timestamps.

## A note on Vertex

Vertex authenticates with short-lived access tokens, so a `vertex` provider configured here needs its
`access_token` refreshed. For Google Cloud, a self-hosted agent pool is usually the steadier route:
the runner uses your own environment's Application Default Credentials, and InfraDots checks that
each runner sends its calls where this resource says — see
[`infradots_organization_model_provider`](organization_model_provider.md).

## Import

Model providers can be imported using the `organization_name` and the model provider ID, separated by
a colon:

```
$ terraform import infradots_model_provider.example infradots:a1b2c3d4-e5f6-7890-abcd-ef1234567890
```

`settings` and `api_key` are write-only and cannot be recovered on import; they will be empty in
state afterwards, and the next apply will send whatever your configuration holds.
