terraform {
  required_providers {
    infradots = {
      source = "infradots/infradots"
    }
  }
}

provider "infradots" {
  host  = "localhost:8000" 
  token = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJJbmZyYWRvdHMiLCJzdWIiOiJhdGFuYXNAaW5mcmFkb3RzLmNvbSAoVXNlciBUb2tlbikiLCJhdWQiOiJJbmZyYWRvdHMiLCJqdGkiOiI2MDhlYzMwZS05NTk4LTRjNjMtYTQ3MC1mMzdkYTNiN2I4MDQiLCJlbWFpbCI6ImF0YW5hc0BpbmZyYWRvdHMuY29tIiwiZGVzY3JpcHRpb24iOiJ0ZXN0IGlkcCBwcm92aWRlciIsImVtYWlsX3ZlcmlmaWVkIjp0cnVlLCJuYW1lIjoiYXRhbmFzQGluZnJhZG90cy5jb20gKFRva2VuKSIsImdyb3VwcyI6WyJkZXZvcHMiXSwiaWF0IjoxNzUxODg4NDM3LCJleHAiOjE3NTQ0ODA0Mzd9.IliMV00bXLayh7B-iMaHV6USz5MCCr5ES8RpZPzDiQ4"
}

# Create resources that we'll reference with data sources
resource "infradots_organization" "example" {
  name = "example-org"
}

resource "infradots_workspace" "example" {
  organization_name = infradots_organization.example.name
  name              = "example-workspace"
  description       = "Example workspace for data source demonstration"
  source            = "https://github.com/example/terraform-config"
  branch            = "main"
  terraform_version = "1.5.0"
}

# Data source examples - retrieving existing resources
data "infradots_organization_data" "by_name" {
  name = infradots_organization.example.name
  
  # Adding a dependency to ensure the resource is created first
  depends_on = [infradots_organization.example]
}

data "infradots_organization_data" "by_id" {
  id = infradots_organization.example.id
  
  # Adding a dependency to ensure the resource is created first
  depends_on = [infradots_organization.example]
}

data "infradots_workspace_data" "by_name" {
  organization_name = infradots_organization.example.name
  name              = infradots_workspace.example.name
  
  # Adding a dependency to ensure the resource is created first
  depends_on = [infradots_workspace.example]
}

data "infradots_workspace_data" "by_id" {
  id                = infradots_workspace.example.id
  organization_name = infradots_organization.example.name
  
  # Adding a dependency to ensure the resource is created first
  depends_on = [infradots_workspace.example]
}

# Creating resources that reference data from the data sources
resource "infradots_variable" "example_from_org" {
  organization_name = data.infradots_organization_data.by_name.name
  key               = "org_id"
  value             = data.infradots_organization_data.by_id.id
  description       = "Organization ID from data source"
  category          = "terraform"
}

resource "infradots_variable" "example_from_workspace" {
  organization_name = data.infradots_workspace_data.by_name.organization_name
  key               = "workspace_branch"
  value             = data.infradots_workspace_data.by_id.branch
  description       = "Branch from workspace data source"
  category          = "terraform"
}

# Output values from data sources
output "organization_by_name" {
  value = {
    id              = data.infradots_organization_data.by_name.id
    name            = data.infradots_organization_data.by_name.name
    created_at      = data.infradots_organization_data.by_name.created_at
    execution_mode  = data.infradots_organization_data.by_name.execution_mode
    agents_enabled  = data.infradots_organization_data.by_name.agents_enabled
    member_count    = length(data.infradots_organization_data.by_name.members)
    team_count      = length(data.infradots_organization_data.by_name.teams)
  }
}

output "workspace_by_name" {
  value = {
    id                = data.infradots_workspace_data.by_name.id
    organization_name = data.infradots_workspace_data.by_name.organization_name
    name              = data.infradots_workspace_data.by_name.name
    description       = data.infradots_workspace_data.by_name.description
    source            = data.infradots_workspace_data.by_name.source
    terraform_version = data.infradots_workspace_data.by_name.terraform_version
  }
}
