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

resource "infradots_vcs" "github" {
  organization_name = infradots_organization.example.name
  name              = "github-connection"
  vcs_type          = "github"
  url               = "https://github.com"
  clientId          = "your_github_oauth_client_id"
  clientSecret      = "your_github_oauth_client_secret"
  description       = "GitHub VCS connection for our organization"
}

resource "infradots_vcs" "gitlab" {
  organization_name = infradots_organization.example.name
  name              = "gitlab-connection"
  vcs_type          = "gitlab"
  url               = "https://gitlab.com"
  clientId          = "your_gitlab_oauth_client_id"
  clientSecret      = "your_gitlab_oauth_client_secret"
  description       = "GitLab VCS connection for our organization"
}

# Data source examples - retrieving existing VCS connections
data "infradots_vcs_data" "github_by_name" {
  organization_name = infradots_organization.example.name
  name              = infradots_vcs.github.name
  
  # Adding a dependency to ensure the resource is created first
  depends_on = [infradots_vcs.github]
}

data "infradots_vcs_data" "github_by_id" {
  id                = infradots_vcs.github.id
  organization_name = infradots_organization.example.name
  
  # Adding a dependency to ensure the resource is created first
  depends_on = [infradots_vcs.github]
}

data "infradots_vcs_data" "gitlab_by_name" {
  organization_name = infradots_organization.example.name
  name              = infradots_vcs.gitlab.name
  
  # Adding a dependency to ensure the resource is created first
  depends_on = [infradots_vcs.gitlab]
}

# Creating workspaces that reference VCS connections from data sources
resource "infradots_workspace" "github_workspace" {
  organization_name = data.infradots_vcs_data.github_by_name.organization_name
  name              = "github-workspace"
  description       = "Workspace using GitHub VCS connection"
  source            = "https://github.com/example/terraform-config"
  branch            = "main"
  terraform_version = "1.5.0"
}

resource "infradots_workspace" "gitlab_workspace" {
  organization_name = data.infradots_vcs_data.gitlab_by_name.organization_name
  name              = "gitlab-workspace"
  description       = "Workspace using GitLab VCS connection"
  source            = "https://gitlab.com/example/terraform-config"
  branch            = "main"
  terraform_version = "1.5.0"
}

# Output values from data sources
output "github_vcs_by_name" {
  value = {
    id               = data.infradots_vcs_data.github_by_name.id
    name             = data.infradots_vcs_data.github_by_name.name
    vcs_type         = data.infradots_vcs_data.github_by_name.vcs_type
    url              = data.infradots_vcs_data.github_by_name.url
    description      = data.infradots_vcs_data.github_by_name.description
    created_at       = data.infradots_vcs_data.github_by_name.created_at
  }
}

output "github_vcs_by_id" {
  value = {
    id               = data.infradots_vcs_data.github_by_id.id
    name             = data.infradots_vcs_data.github_by_id.name
    vcs_type         = data.infradots_vcs_data.github_by_id.vcs_type
    url              = data.infradots_vcs_data.github_by_id.url
    description      = data.infradots_vcs_data.github_by_id.description
    created_at       = data.infradots_vcs_data.github_by_id.created_at
  }
}

output "gitlab_vcs_by_name" {
  value = {
    id               = data.infradots_vcs_data.gitlab_by_name.id
    name             = data.infradots_vcs_data.gitlab_by_name.name
    vcs_type         = data.infradots_vcs_data.gitlab_by_name.vcs_type
    url              = data.infradots_vcs_data.gitlab_by_name.url
    description      = data.infradots_vcs_data.gitlab_by_name.description
    created_at       = data.infradots_vcs_data.gitlab_by_name.created_at
  }
}
