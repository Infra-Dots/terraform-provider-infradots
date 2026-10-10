# SSH Key Data Source

An organization's SSH key, by name: for example, one added in the web app, to set `ssh_key_id` on a workspace.
The private key is never returned.

## Example Usage

```hcl
data "infradots_ssh_key" "modules" {
  organization_name = "acme"
  name              = "private-modules"
}

resource "infradots_workspace" "app" {
  # ...
  ssh_key_id = data.infradots_ssh_key.modules.id
}
```

## Argument Reference

* `organization_name` - (Required) The organization.
* `name` - (Required) The key's name.

## Attributes Reference

* `id` - The SSH key's ID.
* `public_key` - The key's public half, in OpenSSH format.
* `fingerprint` - The key's SHA256 fingerprint.
* `known_hosts` - The key's `known_hosts` lines.
