# Worker Pool Data Source

Use this data source to retrieve details of an existing worker pool in Infradots.

## Example Usage

```hcl
# Retrieve worker pool by name and organization name
data "infradots_worker_pool_data" "by_name" {
  organization_name = "example-org"
  name              = "default-pool"
}

# Retrieve worker pool by ID (organization name is still required)
data "infradots_worker_pool_data" "by_id" {
  organization_name = "example-org"
  id                = "3f340e3c-89f1-4321-bcde-eff34567890a"
}
```

## Argument Reference

* `organization_name` - (Required) The name of the organization the worker pool belongs to.
* `id` - (Optional) The unique ID of the worker pool to retrieve. Either `id` or `name` must be specified.
* `name` - (Optional) The name of the worker pool to retrieve. Either `id` or `name` must be specified.

## Attributes Reference

* `id` - The unique ID of the worker pool.
* `organization_name` - The name of the organization the worker pool belongs to.
* `name` - The name of the worker pool.
* `restrict_to_assigned` - Whether this pool is restricted to assigned workspaces.
* `workers_count` - Number of workers currently registered in this pool.
