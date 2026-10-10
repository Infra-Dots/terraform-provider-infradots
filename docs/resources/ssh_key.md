# SSH Key Resource

An organization's SSH key for **module sources fetched over SSH**: `git::ssh://git@github.com/acme/modules.git//vpc`,
`git@github.com:acme/modules.git`. A workspace uses it with `ssh_key_id`. Each of its jobs hands the key to the executor
that runs it, which writes it to a private file for the length of the run and removes it afterwards.

The private key is **write-only**: InfraDots encrypts it and never returns it. Only its public half and fingerprint are
readable. Executors already trust the host keys of github.com, gitlab.com and bitbucket.org. For other SSH hosts,
add their lines to `known_hosts`.

## Example Usage

```hcl
resource "tls_private_key" "modules" {
  algorithm = "ED25519"
}

resource "infradots_ssh_key" "modules" {
  organization_name = infradots_organization.example.name
  name              = "private-modules"
  private_key       = tls_private_key.modules.private_key_openssh
}

# Add infradots_ssh_key.modules.public_key as a read-only deploy key on the module repositories.

resource "infradots_workspace" "app" {
  organization_name = infradots_organization.example.name
  name              = "app"
  # ...
  ssh_key_id = infradots_ssh_key.modules.id
}
```

A self-hosted git server:

```hcl
resource "infradots_ssh_key" "internal" {
  organization_name = infradots_organization.example.name
  name              = "internal-git"
  private_key       = file("~/.ssh/infradots_modules")
  known_hosts       = "git.acme.internal ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAA..."
}
```

## Argument Reference

* `organization_name` - (Required) The organization the key belongs to. Changing it replaces the key.
* `name` - (Required) The key's name, unique in the organization.
* `private_key` - (Required, Sensitive) The private key, in OpenSSH or PEM format, **without a passphrase**: an executor
  can't enter one. Write-only. Changing it rotates the key.
* `known_hosts` - (Optional) `known_hosts` lines for SSH hosts other than github.com, gitlab.com and bitbucket.org.

## Attributes Reference

* `id` - The SSH key's ID.
* `public_key` - The key's public half, in OpenSSH format. Add it as a deploy key wherever the modules live.
* `fingerprint` - The key's SHA256 fingerprint, as `ssh-keygen -lf` prints it.

## Import

```shell
terraform import infradots_ssh_key.modules acme:<id>
```

The private key can't be imported. Keep `private_key` in the configuration, and the first apply after the import
sets the key to it.
