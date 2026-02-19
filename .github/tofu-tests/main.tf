terraform {
  required_providers {
    infradots = {
      source  = "infra-dots/infradots"
      version = ">= 1.0.0"
    }
  }
}

provider "infradots" {
  token    = var.infradots_token
  hostname = var.infradots_hostname
}

locals {
  run_id = formatdate("YYYYMMDDhhmmss", timestamp())
}

# ──────────────────────────────────────────────
# VCS
# ──────────────────────────────────────────────

resource "infradots_vcs" "test_vcs" {
  organization_name = var.organization_name
  name              = "test-vcs-integration-${local.run_id}"
  vcs_type          = "github"
  url               = "https://github.com"
  client_id         = "test-client-id"
  client_secret     = "test-client-secret"
  description       = "Test VCS connection for integration tests"
}

# ──────────────────────────────────────────────
# Workspace (with new fields)
# ──────────────────────────────────────────────

resource "infradots_workspace" "test_workspace" {
  organization_name = var.organization_name
  name              = "test-ws-${local.run_id}"
  description       = "Test workspace for integration tests"
  source            = "https://github.com/example/terraform-config"
  branch            = "main"
  terraform_version = "1.5.0"

  auto_apply        = false
  iac_type          = "TF"
  default_job_action = "plan"
  folder            = "/"
  execution_mode    = "Remote"
  locked            = false
  agents_enabled    = false
  tags = {
    environment = "integration-test"
    managed_by  = "tofu"
  }
}

resource "infradots_workspace" "test_workspace_b" {
  organization_name = var.organization_name
  name              = "test-ws-b-${local.run_id}"
  description       = "Second workspace for interconnection tests"
  source            = "https://github.com/example/terraform-config"
  branch            = "main"
  terraform_version = "1.5.0"
  execution_mode    = "Remote"
}

# ──────────────────────────────────────────────
# Variable
# ──────────────────────────────────────────────

resource "infradots_variable" "test_workspace_variable" {
  organization_name = var.organization_name
  workspace         = infradots_workspace.test_workspace.name
  key               = "test_ws_var_${local.run_id}"
  value             = "test-workspace-value"
  description       = "Test workspace variable for integration tests"
  category          = "terraform"
  sensitive         = false
  hcl               = false
}

# ──────────────────────────────────────────────
# Team
# ──────────────────────────────────────────────

resource "infradots_team" "test_team" {
  organization_name = var.organization_name
  name              = "test-team-${local.run_id}"
}

data "infradots_team_data" "lookup_team" {
  organization_name = var.organization_name
  name              = infradots_team.test_team.name

  depends_on = [infradots_team.test_team]
}

# ──────────────────────────────────────────────
# Permission (org-level, assigned to team)
# ──────────────────────────────────────────────

resource "infradots_permission" "team_read_workspaces" {
  organization_name = var.organization_name
  team_id           = infradots_team.test_team.id
  permission        = "read_workspaces"
}

resource "infradots_permission" "team_write_workspaces" {
  organization_name = var.organization_name
  team_id           = infradots_team.test_team.id
  permission        = "write_workspaces"
}

# ──────────────────────────────────────────────
# Worker Pool
# ──────────────────────────────────────────────

resource "infradots_worker_pool" "test_pool" {
  organization_name  = var.organization_name
  name               = "test-pool-${local.run_id}"
  restrict_to_assigned = false
}

data "infradots_worker_pool_data" "lookup_pool" {
  organization_name = var.organization_name
  name              = infradots_worker_pool.test_pool.name

  depends_on = [infradots_worker_pool.test_pool]
}

# ──────────────────────────────────────────────
# Workspace Interconnection
# ──────────────────────────────────────────────

resource "infradots_workspace_interconnection" "test_interconnection" {
  organization_name = var.organization_name
  workspace_name    = infradots_workspace.test_workspace.name
  connected_to      = [infradots_workspace.test_workspace_b.name]
  condition         = "full_apply"
}

# ──────────────────────────────────────────────
# Outputs
# ──────────────────────────────────────────────

output "team_id" {
  value = infradots_team.test_team.id
}

output "team_name_from_data" {
  value = data.infradots_team_data.lookup_team.name
}

output "worker_pool_id" {
  value = infradots_worker_pool.test_pool.id
}

output "worker_pool_workers_count" {
  value = data.infradots_worker_pool_data.lookup_pool.workers_count
}

output "workspace_tags" {
  value = infradots_workspace.test_workspace.tags
}

output "workspace_interconnection_id" {
  value = infradots_workspace_interconnection.test_interconnection.id
}

