# Integration Tests

This directory contains integration tests for the Infradots Terraform provider.

## Resources Created

The integration tests create the following resources:

1. **VCS Connection** (`infradots_vcs.test_vcs`)
   - A GitHub VCS connection for testing

2. **Workspace** (`infradots_workspace.test_workspace`)
   - A test workspace with basic configuration

3. **Workspace Variable** (`infradots_variable.test_workspace_variable`)
   - A workspace-level variable associated with the test workspace

## Running Locally

To run these tests locally, you'll need:

1. OpenTofu installed
2. The provider built and available in the local registry
3. Environment variables or a `terraform.tfvars` file with:
   - `organization_name`: Your Infradots organization name
   - `infradots_token`: Your Infradots API token
   - `infradots_hostname`: (Optional) Infradots API hostname (defaults to `api.infradots.com`)

Example:

```bash
# Build the provider
go build -o terraform-provider-infradots

# Setup provider in local registry
mkdir -p ~/.terraform.d/plugins/registry.terraform.io/local/infra-dots/infradots/0.1.0/linux_amd64/
cp terraform-provider-infradots ~/.terraform.d/plugins/registry.terraform.io/local/infra-dots/infradots/0.1.0/linux_amd64/

# Initialize and run
cd .github/tofu-tests
tofu init
tofu apply -var="organization_name=your-org" -var="infradots_token=your-token"
tofu destroy -var="organization_name=your-org" -var="infradots_token=your-token"
```

## GitHub Secrets

The GitHub workflow requires the following secrets:

- `INFRADOTS_ORGANIZATION_NAME`: The name of your Infradots organization
- `INFRADOTS_TOKEN`: Your Infradots API token
- `INFRADOTS_HOSTNAME`: (Optional) The Infradots API hostname (defaults to `api.infradots.com`)

