terraform {
  required_providers {
    infradots = {
      source = "infradots/infradots"
    }
  }
}

provider "infradots" {
  token = "idp-token"
}

resource "infradots_organization" "example" {
  name = "example-org"
}

output "organization_id" {
  value = infradots_organization.example.id
}


output "agents_enabled" {
  value = infradots_organization.example.agents_enabled
}

output "execution_mode" {
  value = infradots_organization.example.execution_mode
}
