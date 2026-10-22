# Organization Model Provider Resource

Points the organization's agents at one of its own [model providers](model_provider.md): the same
Claude models InfraDots runs, billed to your account instead of ours. Without this resource, an
organization's agents run on InfraDots' own model account.

It's a resource of its own, not an argument of `infradots_organization`, because the provider belongs
to the organization: setting it on the organization would make the two depend on each other — the
same reason [`infradots_organization_agent_pool`](organization_agent_pool.md) is separate.

Destroying it sends the organization's model calls back to InfraDots' own account.

## Example Usage

```hcl
resource "infradots_model_provider" "bedrock" {
  organization_name = infradots_organization.example.name
  name              = "our-bedrock"
  provider_type     = "bedrock"

  settings = {
    aws_region     = "eu-west-1"
    aws_access_key = var.aws_access_key
    aws_secret_key = var.aws_secret_key
  }

  model_map = {
    "claude-opus-5"   = "eu.anthropic.claude-opus-5-v1:0"
    "claude-sonnet-5" = "eu.anthropic.claude-sonnet-5-v1:0"
  }
}

resource "infradots_organization_model_provider" "example" {
  organization_name = infradots_organization.example.name
  model_provider_id = infradots_model_provider.bedrock.id
}
```

## Argument Reference

* `organization_name` - (Required) The organization. Changing it replaces the resource.
* `model_provider_id` - (Required) ID of one of the organization's own `infradots_model_provider`
  resources. It has to be enabled, and to carry the credentials that identify its account.
* `allow_platform_fallback` - (Optional) When the provider cannot be reached, run on InfraDots'
  model instead of failing the run. Defaults to `false`.

  Leaving it off is the safe choice, and the reason is billing rather than reliability: a provider
  that silently falls back would run your agents on InfraDots' model while you believed you were
  paying your own model bill. With it off, a broken provider fails the run and says what to fix.

## Attributes Reference

* `id` - The organization name.

## Runs on your own agent pool

A run queued to an [agent pool](organization_agent_pool.md) is routed by the runner that claims it,
from that runner's own `MODEL_PROVIDER` and `MODEL_MAP` environment — a runner holds its own
credentials, which is the point of a self-hosted pool, and for Vertex they are the only ones that can
work.

Pointing an organization here still matters for those runs. InfraDots compares each runner's
registered provider against what this resource says and reports a runner that disagrees, on the
pool's page, before a run fails. And a run queued to InfraDots' *shared* agent pool, whose runners
hold InfraDots' credentials, is refused while this resource is set and `allow_platform_fallback` is
`false` — that combination is a fallback like any other.

## Import

```
$ terraform import infradots_organization_model_provider.example infradots
```
