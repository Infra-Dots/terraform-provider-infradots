# Service Account Resource

The service account resource allows you to create and manage service accounts in the Infradots platform. This is an admin-only resource.

## Example Usage

```hcl
resource "infradots_service_account" "example" {
  name        = "ci-pipeline"
  description = "Service account used by the CI/CD pipeline"
  scopes = [
    "read_workspaces",
    "write_workspaces",
  ]
  is_active = true
}
```

## Argument Reference

The following arguments are supported:

* `name` - (Required) The unique name of the service account.
* `description` - (Optional) A description of the service account.
* `scopes` - (Optional) A list of scopes assigned to the service account.
* `is_active` - (Optional) Whether the service account is active. Defaults to `true`.

## Attributes Reference

In addition to all arguments above, the following attributes are exported:

* `id` - The service account unique ID (UUID).
* `created_at` - The timestamp when the service account was created (RFC3339 format).

## Import

Service accounts can be imported using their `id` (UUID), e.g.,

```
$ terraform import infradots_service_account.example 2e240d2c-78e0-4832-abdc-daa33477a238
```
