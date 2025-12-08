# VCS Resource

The VCS resource allows you to create and manage VCS (Version Control System) connections within organizations in Infradots.

## Example Usage

```hcl
resource "infradots_vcs" "example" {
  organization_name = "infradots"
  name              = "github-connection"
  vcs_type          = "github"
  url               = "https://github.com"
  client_id          = "your_oauth_client_id"
  client_secret      = "your_oauth_client_secret"
  description       = "GitHub VCS connection for our organization"
}
```

## Argument Reference

The following arguments are supported:

* `organization_name` - (Required) The name of the organization this VCS connection belongs to.
* `name` - (Required) The name of the VCS connection.
* `vcs_type` - (Required) The type of VCS (e.g., "github", "gitlab", "bitbucket").
* `url` - (Required) The URL of the VCS instance.
* `client_id` - (Required) The OAuth client ID for the VCS.
* `client_secret` - (Required, Sensitive) The OAuth client secret for the VCS.
* `description` - (Optional) A description of the VCS connection. Defaults to an empty string.

## Attributes Reference

In addition to all arguments above, the following attributes are exported:

* `id` - The ID of the VCS connection (UUID).
* `created_at` - The timestamp when the VCS connection was created (RFC3339 format).
* `updated_at` - The timestamp when the VCS connection was last updated (RFC3339 format).

## Import

VCS connections can be imported using the `organization_name` and `id` separated by a colon, e.g.,

```
$ terraform import infradots_vcs.example infradots:5f560f5e-0bf3-6543-defg-g1156789012c
```
