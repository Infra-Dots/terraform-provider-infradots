terraform {
  required_providers {
    infradots = {
      source = "infradots/infradots"
    }
  }
}

provider "infradots" {
  host  = "api.infradots.com"  # Using the provided localhost URL
  token = "idp-token"
}

resource "infradots_organization" "example" {
  name = "example-org"
}

output "organization_id" {
  value = infradots_organization.example.id
}

output "organization_created_at" {
  value = infradots_organization.example.created_at
}

output "organization_execution_mode" {
  value = infradots_organization.example.execution_mode
}
