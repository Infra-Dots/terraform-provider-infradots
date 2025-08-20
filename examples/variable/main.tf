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

# Create a simple Terraform variable
resource "infradots_variable" "tf_var" {
  organization_name = infradots_organization.example.name
  key               = "region"
  value             = "us-west-2"
  description       = "AWS region for deployment"
  category          = "terraform"
  sensitive         = false
  hcl               = false
}

# Create an environment variable (sensitive)
resource "infradots_variable" "env_var" {
  organization_name = infradots_organization.example.name
  key               = "AWS_ACCESS_KEY_ID"
  value             = "AKIAIOSFODNN7EXAMPLE"
  description       = "AWS access key for the deployment"
  category          = "env"
  sensitive         = true
  hcl               = false
}

# Create a variable with HCL value
resource "infradots_variable" "hcl_var" {
  organization_name = infradots_organization.example.name
  key               = "availability_zones"
  value             = <<EOT
[
  "us-west-2a",
  "us-west-2b",
  "us-west-2c"
]
EOT
  description       = "List of availability zones"
  category          = "terraform"
  hcl               = true
}

# Output variable IDs
output "tf_var_id" {
  value = infradots_variable.tf_var.id
}

output "env_var_id" {
  value = infradots_variable.env_var.id
}

output "hcl_var_id" {
  value = infradots_variable.hcl_var.id
}
