# Service Account Token Resource

The service account token resource allows you to create and manage tokens for a service account in the Infradots platform. This is an admin-only resource.

## Example Usage

```hcl
resource "infradots_service_account_token" "example" {
  service_account_id = infradots_service_account.example.id
  description        = "CI/CD pipeline token"
  expiration         = "2027-01-01T00:00:00Z"
}
```

## Argument Reference

The following arguments are supported:

* `service_account_id` - (Required) The ID of the service account this token belongs to. Changing this forces a new token to be created.
* `description` - (Required) A description of the token.
* `expiration` - (Optional) The expiration date of the token (RFC3339 format). If omitted, the token does not expire. Changing this forces a new token to be created.

## Attributes Reference

In addition to all arguments above, the following attributes are exported:

* `id` - The token unique ID (UUID).
* `created_at` - The timestamp when the token was created (RFC3339 format).
* `last_used` - The timestamp when the token was last used (RFC3339 format).
* `jwt` - The JWT value of the token. This is marked as sensitive and is only available at creation time.

## Import

Service account tokens can be imported using the `service_account_id` and `token_id` separated by a colon, e.g.,

```
$ terraform import infradots_service_account_token.example 2e240d2c-78e0-4832-abdc-daa33477a238:9f3b1a7e-4c2d-4e8a-bb11-77c0d9e2f001
```
