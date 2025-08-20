# Variable Resource

The variable resource allows you to create and manage variables within organizations in Infradots.

## Example Usage

```hcl
resource "infradots_variable" "example" {
  organization_name = "infradots"
  key               = "environment"
  value             = "production"
  description       = "Environment type for deployment"
  category          = "terraform"  # "terraform" or "env"
  sensitive         = false
  hcl               = false
}
```

## Argument Reference

The following arguments are supported:

* `organization_name` - (Required) The name of the organization this variable belongs to.
* `key` - (Required) The name of the variable.
* `value` - (Required) The value of the variable. This is always marked as sensitive in state.
* `description` - (Optional) A description of the variable. Defaults to an empty string.
* `category` - (Optional) The category of the variable. Valid values are "terraform" or "env". Defaults to "terraform".
* `sensitive` - (Optional) Whether the variable contains sensitive information. Defaults to false.
* `hcl` - (Optional) Whether to parse the value as HCL. Defaults to false.

## Attributes Reference

In addition to all arguments above, the following attributes are exported:

* `id` - The ID of the variable (UUID).
* `created_at` - The timestamp when the variable was created (RFC3339 format).
* `updated_at` - The timestamp when the variable was last updated (RFC3339 format).

## Import

Variables can be imported using the `organization_name` and `id` separated by a colon, e.g.,

```
$ terraform import infradots_variable.example infradots:2e240d2c-78e0-4832-abdc-daa33477a238
