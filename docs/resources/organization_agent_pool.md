# Organization Agent Pool Resource

Sets the organization's **agent pool**: where its AI reviews and implementations run, on your own runners and with
your own model account, instead of on InfraDots. A workspace can still choose its own pool with
`infradots_workspace.agent_pool_id`, which takes precedence.

It's a resource of its own, not an argument of `infradots_organization`, because the pool belongs to the
organization: setting it on the organization would make the two depend on each other.

Destroying it clears the organization's agent pool: agent runs go back to InfraDots.

## Example Usage

```hcl
resource "infradots_worker_pool" "agents" {
  organization_name = infradots_organization.example.name
  name              = "k8s-agents"
  kind              = "agent"
}

resource "infradots_organization_agent_pool" "example" {
  organization_name = infradots_organization.example.name
  agent_pool_id     = infradots_worker_pool.agents.id
}

# Start the runners with the pool's token, e.g. the infradots-runner Helm chart.
output "agent_pool_token" {
  value     = infradots_worker_pool.agents.registration_token
  sensitive = true
}
```

## Argument Reference

* `organization_name` - (Required) The organization. Changing it moves the setting to another organization.
* `agent_pool_id` - (Required) ID of one of the organization's agent pools (`infradots_worker_pool` with `kind = "agent"`). An executor pool, or another organization's pool, is rejected.

Setting it requires the organization's write permission (organization admins have it).

## Attributes Reference

* `id` - The organization name.

## Import

Import by organization name:

```
$ terraform import infradots_organization_agent_pool.example infradots
```
