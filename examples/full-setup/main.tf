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

# Create the organization
resource "infradots_organization" "main" {
  name = "my-company"
}

# Workspace configurations
locals {
  workspaces = {
    dev = {
      description = "Development environment workspace"
      branch      = "develop"
    }
    stage = {
      description = "Staging environment workspace"
      branch      = "staging"
    }
    live = {
      description = "Production environment workspace"
      branch      = "main"
    }
  }

  # Variable configurations for each environment
  environment_variables = {
    for env in keys(local.workspaces) : env => {
      aws_access_key = {
        key         = "${env}_aws_access_key"
        value       = "AKIAIOSFODNN7${upper(env)}EXAMPLE"
        description = "AWS access key for ${env} environment"
      }
      aws_secret_key = {
        key         = "${env}_aws_secret_key"
        value       = "wJalrXUtnFEMI/K7MDENG/bPxRfiCY${upper(env)}EXAMPLEKEY"
        description = "AWS secret key for ${env} environment"
      }
    }
  }

  # Flatten the variables for for_each
  all_variables = merge([
    for env, vars in local.environment_variables : {
      for var_name, var_config in vars :
      "${env}_${var_name}" => merge(var_config, { environment = env })
    }
  ]...)
}

# Create workspaces using for_each
resource "infradots_workspace" "environments" {
  for_each = local.workspaces
  
  organization_name   = infradots_organization.main.name
  name              = each.key
  description       = each.value.description
  source            = "https://github.com/my-company/terraform-infrastructure"
  branch            = each.value.branch
  terraform_version = "1.5.0"
}

# Create variables using for_each
resource "infradots_variable" "environment_vars" {
  for_each = local.all_variables
  
  organization_name = infradots_organization.main.name
  key               = each.value.key
  value             = each.value.value
  description       = each.value.description
  category          = "env"
  sensitive         = true
  hcl               = false
}

# Outputs
output "organization_id" {
  value = infradots_organization.main.id
}

output "organization_name" {
  value = infradots_organization.main.name
}

output "workspace_ids" {
  value = {
    for k, v in infradots_workspace.environments : k => v.id
  }
}

output "workspace_created_at" {
  value = {
    for k, v in infradots_workspace.environments : k => v.created_at
  }
}

output "variable_ids" {
  value = {
    for k, v in infradots_variable.environment_vars : k => v.id
  }
}
