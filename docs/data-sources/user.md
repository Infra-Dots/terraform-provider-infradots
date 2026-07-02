# User Data Source

Use this data source to retrieve details of an existing user in Infradots.

## Example Usage

```hcl
# Retrieve a user by email address within an organization
data "infradots_user_data" "example" {
  organization_name = "example-org"
  email             = "jane.doe@example.com"
}
```

## Argument Reference

* `organization_name` - (Required) The name of the organization the user belongs to.
* `email` - (Required) The email address of the user to look up.

## Attributes Reference

* `id` - The unique ID of the user.
* `organization_name` - The name of the organization the user belongs to.
* `email` - The email address of the user.
* `name` - The name of the user.
