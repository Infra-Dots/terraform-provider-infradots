# Team Data Source

Use this data source to retrieve details of an existing team in Infradots.

## Example Usage

```hcl
# Retrieve team by name and organization name
data "infradots_team_data" "by_name" {
  organization_name = "example-org"
  name              = "platform-team"
}

# Retrieve team by ID (organization name is still required)
data "infradots_team_data" "by_id" {
  organization_name = "example-org"
  id                = "3f340e3c-89f1-4321-bcde-eff34567890a"
}
```

## Argument Reference

* `organization_name` - (Required) The name of the organization the team belongs to.
* `id` - (Optional) The unique ID of the team to retrieve. Either `id` or `name` must be specified.
* `name` - (Optional) The name of the team to retrieve. Either `id` or `name` must be specified.

## Attributes Reference

* `id` - The unique ID of the team.
* `organization_name` - The name of the organization the team belongs to.
* `name` - The name of the team.
* `members` - List of member email addresses in the team.
