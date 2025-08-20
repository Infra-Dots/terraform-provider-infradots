# Terraform Provider for InfraDots

A Terraform provider for managing resources on the [InfraDots platform](https://infradots.com) - a comprehensive Infrastructure as Code management platform that streamlines your DevOps workflows.

## Features

This provider allows you to manage InfraDots resources using Terraform or OpenTofu, including:

- **Organizations** - Create and manage organizational units
- **Workspaces** - Manage Terraform/OpenTofu workspaces with Git integration
- **Variables** - Configure workspace variables

## Requirements

- [Terraform](https://www.terraform.io/downloads.html) >= 1.0 or [OpenTofu](https://opentofu.org/) >= 1.6
- An active InfraDots account and API token

## Installation

### Terraform Registry (Recommended)

Add the provider to your Terraform configuration:

```hcl
terraform {
  required_providers {
    infradots = {
      source  = "infradots/infradots"
      version = "~> 1.0"
    }
  }
}
```

### Local Development

For local development, use the provided script:

```bash
./make-local-provider.sh
```

## Configuration

Configure the provider with your InfraDots instance details:

```hcl
provider "infradots" {
  hostname = "api.infradots.com"  # Your InfraDots instance hostname
  token    = var.infradots_token  # Your API token (recommended to use variables)
  
  # Optional: Skip TLS verification (not recommended for production)
  # tls_insecure_skip_verify = false
}
```

### Environment Variables

You can also configure the provider using environment variables:

- `INFRADOTS_HOSTNAME` - InfraDots instance hostname
- `INFRADOTS_TOKEN` - API token
- `INFRADOTS_TLS_INSECURE_SKIP_VERIFY` - Skip TLS verification (true/false)

## Usage Example

```hcl
# Create an organization
resource "infradots_organization" "my_org" {
  name = "my-organization"
}

# Create a workspace
resource "infradots_workspace" "my_workspace" {
  organization_id   = infradots_organization.my_org.id
  name              = "production-infrastructure"
  description       = "Production environment infrastructure"
  source            = "https://github.com/my-org/infrastructure.git"
  branch            = "main"
  terraform_version = "1.5.2"
}

# Configure workspace variables
resource "infradots_variable" "aws_region" {
  workspace_id = infradots_workspace.my_workspace.id
  key          = "AWS_REGION"
  value        = "us-east-1"
  category     = "env"
  sensitive    = false
}

resource "infradots_variable" "aws_access_key" {
  workspace_id = infradots_workspace.my_workspace.id
  key          = "AWS_ACCESS_KEY_ID"
  value        = var.aws_access_key_id
  category     = "env"
  sensitive    = true
}

# Query existing organizations
data "infradots_organization" "existing" {
  name = "existing-org"
}

# Query existing workspaces
data "infradots_workspace" "existing" {
  organization_id = data.infradots_organization.existing.id
  name            = "existing-workspace"
}
```

## Resources

| Resource | Description |
|----------|-------------|
| [`infradots_organization`](docs/resources/organization.md) | Manage organizations |
| [`infradots_workspace`](docs/resources/workspace.md) | Manage workspaces |
| [`infradots_variable`](docs/resources/variable.md) | Manage workspace variables |

## Data Sources

| Data Source | Description |
|-------------|-------------|
| [`infradots_organization`](docs/data-sources/organization.md) | Query organizations |
| [`infradots_workspace`](docs/data-sources/workspace.md) | Query workspaces |

## Documentation

- [Provider Documentation](docs/index.md)
- [Resource Documentation](docs/resources/)
- [Data Source Documentation](docs/data-sources/)
- [Examples](examples/)

## OpenTofu Compatibility

This provider is fully compatible with OpenTofu. Simply replace `terraform` with `tofu` in your commands:

```bash
tofu init
tofu plan
tofu apply
```

## Development

### Building the Provider

```bash
go build -o terraform-provider-infradots
```

### Running Tests

```bash
go test ./...
```

### Local Testing

1. Build the provider locally
2. Run `./make-local-provider.sh` to set up local development
3. Navigate to the `examples/` directory
4. Run `terraform init && terraform plan`

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests for new functionality
5. Run `go test ./...` to ensure tests pass
6. Submit a pull request

## License

This project is licensed under the Mozilla Public License 2.0 - see the [LICENSE](LICENSE) file for details.

## Support

For support and questions:
- [InfraDots Documentation](https://docs.infradots.com)
- [GitHub Issues](https://github.com/infradots/terraform-provider-infradots/issues)
- [InfraDots Support](https://infradots.com/support)
