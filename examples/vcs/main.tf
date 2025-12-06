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

# First, create or reference an organization
resource "infradots_organization" "example" {
  name = "example-org"
}

# Create a GitHub VCS connection
resource "infradots_vcs" "github" {
  organization_name = infradots_organization.example.name
  name              = "github-connection"
  vcs_type          = "github"
  url               = "https://github.com"
  clientId          = "your_github_oauth_client_id"
  clientSecret      = "your_github_oauth_client_secret"
  description       = "GitHub VCS connection for our organization"
}

# Create a GitLab VCS connection
resource "infradots_vcs" "gitlab" {
  organization_name = infradots_organization.example.name
  name              = "gitlab-connection"
  vcs_type          = "gitlab"
  url               = "https://gitlab.com"
  clientId          = "your_gitlab_oauth_client_id"
  clientSecret      = "your_gitlab_oauth_client_secret"
  description       = "GitLab VCS connection for our organization"
}

# Create a Bitbucket VCS connection
resource "infradots_vcs" "bitbucket" {
  organization_name = infradots_organization.example.name
  name              = "bitbucket-connection"
  vcs_type          = "bitbucket"
  url               = "https://bitbucket.org"
  clientId          = "your_bitbucket_oauth_client_id"
  clientSecret      = "your_bitbucket_oauth_client_secret"
  description       = "Bitbucket VCS connection for our organization"
}

# Output VCS connection IDs
output "github_vcs_id" {
  value = infradots_vcs.github.id
}

output "gitlab_vcs_id" {
  value = infradots_vcs.gitlab.id
}

output "bitbucket_vcs_id" {
  value = infradots_vcs.bitbucket.id
}
