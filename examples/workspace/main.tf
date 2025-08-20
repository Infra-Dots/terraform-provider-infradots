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
}

# Output the workspace details
output "workspace_id" {
  value = infradots_workspace.example.id
}

output "workspace_created_at" {
  value = infradots_workspace.example.created_at
}
