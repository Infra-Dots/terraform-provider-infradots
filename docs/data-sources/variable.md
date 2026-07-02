# Variable Data Source

Use this data source to retrieve details of an existing variable in Infradots.

## Example Usage

```hcl
# Retrieve an organization-level variable by key
data "infradots_variable_data" "org_variable" {
  organization_name = "example-org"
  key               = "region"
}

# Retrieve a workspace-level variable by key
data "infradots_variable_data" "workspace_variable" {
  organization_name = "example-org"
  workspace_name    = "example-workspace"
  key               = "environment"
}
```

## Argument Reference

* `organization_name` - (Required) The name of the organization the variable belongs to.
* `key` - (Required) The name/key of the variable to look up.
* `workspace_name` - (Optional) The name of the workspace. If provided, fetches a workspace variable; if absent, fetches an organization variable.

## Attributes Reference

* `id` - The unique ID of the variable.
* `organization_name` - The name of the organization the variable belongs to.
* `workspace_name` - The name of the workspace the variable belongs to, if any.
* `key` - The name/key of the variable.
* `value` - The value of the variable. This is a sensitive value.
* `category` - The category of the variable (e.g., `terraform` or `env`).
* `sensitive` - Whether the variable contains sensitive data.
* `hcl` - Whether the value is parsed as HCL.
* `description` - A description of the variable.
