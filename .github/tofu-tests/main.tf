terraform {
  required_providers {
    infradots = {
      source  = "infra-dots/infradots"
      version = ">= 1.0.0"
    }
  }
}

provider "infradots" {
  token    = var.infradots_token
  hostname = var.infradots_hostname
}

# Create a VCS connection
resource "infradots_vcs" "test_vcs" {
  organization_name = var.organization_name
  name              = "test-vcs-integration-${formatdate("YYYYMMDDhhmmss", timestamp())}"
  vcs_type          = "github"
  url               = "https://github.com"
  client_id         = "test-client-id"
  client_secret     = "test-client-secret"
  description       = "Test VCS connection for integration tests"
}

# Create a workspace
resource "infradots_workspace" "test_workspace" {
  organization_name = var.organization_name
  name              = "test-workspace-integration-${formatdate("YYYYMMDDhhmmss", timestamp())}"
  description       = "Test workspace for integration tests"
  source            = "https://github.com/example/terraform-config"
  branch            = "main"
  terraform_version = "1.5.0"
}

# Create a workspace-level variable
resource "infradots_variable" "test_workspace_variable" {
  organization_name = var.organization_name
  workspace         = infradots_workspace.test_workspace.name
  key               = "test_workspace_var_integration_${formatdate("YYYYMMDDhhmmss", timestamp())}"
  value             = "test-workspace-value"
  description       = "Test workspace variable for integration tests"
  category          = "terraform"
  sensitive         = false
  hcl               = false
}

