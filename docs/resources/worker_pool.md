# Worker Pool Resource

The worker pool resource allows you to create and manage worker pools for remote execution within organizations in Infradots.
A pool's runners run in your own environment and register with its `registration_token`: an **executor** pool runs
Terraform/OpenTofu jobs, an **agent** pool runs AI reviews and implementations with your own model account. See
[Self-hosted runners](https://infradots.com/docs/self-hosted-runners) for running them.

Managing pools requires the organization's write permission (organization admins have it).

## Example Usage

```hcl
resource "infradots_worker_pool" "executors" {
  organization_name    = "infradots"
  name                 = "self-hosted-pool"
  restrict_to_assigned = true
}

resource "infradots_worker_pool" "agents" {
  organization_name = "infradots"
  name              = "k8s-agents"
  kind              = "agent"
}
```

## Argument Reference

The following arguments are supported:

* `organization_name` - (Required) The name of the organization this worker pool belongs to.
* `name` - (Required) The name of the worker pool.
* `kind` - (Optional) What the pool's runners run: `executor` (Terraform/OpenTofu jobs) or `agent` (AI reviews and implementations). Defaults to `executor`. A pool's kind can't change: changing it destroys the pool and creates a new one, with a new registration token.
* `restrict_to_assigned` - (Optional) Whether to restrict this pool to only assigned workspaces. Defaults to `false`.

## Attributes Reference

In addition to all arguments above, the following attributes are exported:

* `id` - The worker pool unique ID (UUID).
* `registration_token` - The registration token for workers to join this pool. Only available after creation and marked as sensitive.

## Import

Worker pools can be imported using the `organization_name` and the pool **name**, separated by a colon:

```
$ terraform import infradots_worker_pool.example infradots:self-hosted-pool
```

Note that `registration_token` is not returned by the API on read and cannot be recovered on import; it will be empty in state afterwards.
