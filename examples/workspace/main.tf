terraform {
  required_providers {
    infradots = {
      source = "infradots/infradots"
    }
  }
}

provider "infradots" {
  host  = "api.infradots.com"
  token = "idp-token"
}

resource "infradots_organization" "example" {
  name = "example-org"
}

# Then create a workspace within the organization
resource "infradots_workspace" "example" {
  organization_name = infradots_organization.example.name
  name              = "example-workspace"
  description       = "Example workspace for Terraform configurations"
  source            = "https://github.com/example/terraform-config"
  branch            = "main"
  terraform_version = "1.5.0"
  folder            = "prod"

  # Also trigger a run when shared modules or root tfvars change, not only the prod/ folder.
  trigger_patterns = [
    { pattern = "modules/.*" },
    { pattern = "^globals\\.tfvars$" },
  ]
}

# A workspace whose repository does not exist yet: infradots creates it on the connected VCS
# rather than leaving a workspace that looks fine and fails on its first clone. Without
# create_repository a missing repository is reported as a warning and the workspace is still made.
resource "infradots_workspace" "bootstrapped" {
  organization_name = infradots_organization.example.name
  name              = "clients-poc-environments"
  source            = "example/clients-poc-environments"
  branch            = "main"
  terraform_version = "1.5.0"

  vcs_id            = "00000000-0000-0000-0000-000000000000"
  create_repository = true
}

# Output the workspace details
output "workspace_id" {
  value = infradots_workspace.example.id
}

output "bootstrapped_repository_created" {
  value = infradots_workspace.bootstrapped.repository_created
}

output "workspace_created_at" {
  value = infradots_workspace.example.created_at
}
