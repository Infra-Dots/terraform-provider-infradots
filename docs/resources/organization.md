# Organization Resource

The organization resource allows you to create and manage organizations in Infradots.

## Example Usage

```hcl
resource "infradots_organization" "example" {
  name           = "example-org"
  execution_mode = "Remote"
  agents_enabled = true
}
```

## Argument Reference

The following arguments are supported:

* `name` - (Required) The unique name of the organization.
* `execution_mode` - (Optional) The execution mode for the organization (e.g., "Remote", "Local"). If not specified, defaults to the system default.
* `agents_enabled` - (Optional) Whether agents are enabled for the organization. If not specified, defaults to the system default.

## Attributes Reference

In addition to all arguments above, the following attributes are exported:

* `id` - The ID of the organization (UUID).
* `created_at` - The timestamp when the organization was created (RFC3339 format).
* `updated_at` - The timestamp when the organization was last updated (RFC3339 format).

## Import

Organizations can be imported using the `id`, e.g.,

```
$ terraform import infradots_organization.example 2e240d2c-78e0-4832-abdc-daa33477a238
