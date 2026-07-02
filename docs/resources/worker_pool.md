# Worker Pool Resource

The worker pool resource allows you to create and manage worker pools for remote execution within organizations in Infradots.

## Example Usage

```hcl
resource "infradots_worker_pool" "example" {
  organization_name    = "infradots"
  name                 = "self-hosted-pool"
  restrict_to_assigned = true
}
```

## Argument Reference

The following arguments are supported:

* `organization_name` - (Required) The name of the organization this worker pool belongs to.
* `name` - (Required) The name of the worker pool.
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
