# Example Terraform configuration
terraform {
  required_providers {
    infradots = {
      source  = "local/infradots/infradots"
      version = "0.0.1"
    }
  }
}

provider "infradots" {
  hostname                 = "api.infradots.com"
  token                    = "my-secret-token"
  tls_insecure_skip_verify = true
}

resource "infradots_organization" "test_org" {
  name = "MyTestOrganization"
}

resource "infradots_workspace" "test_ws" {
  organization_id   = infradots_organization.test_org.id
  name              = "my-workspace"
  description       = "A workspace"
  source            = "https://github.com/example/repo.git"
  branch            = "main"
  terraform_version = "1.5.2"
}
