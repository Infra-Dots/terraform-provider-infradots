# Agent Skill Resource

The agent skill resource allows you to create and manage AI agent skills within organizations in Infradots.

## Example Usage

```hcl
resource "infradots_agent_skill" "review" {
  organization_name = "my-org"
  name              = "review"
  display_name      = "Code Review"
  description       = "Reviews Terraform plans before apply"
  enabled           = true
  config            = jsonencode({
    model       = "claude-sonnet-4-6"
    temperature = 0.3
  })
}

resource "infradots_agent_skill" "custom_bootstrap" {
  organization_name = "my-org"
  name              = "custom-bootstrap"
  display_name      = "Custom Bootstrap"
  description       = "GitHub-sourced bootstrap skill"
  enabled           = true
  source_repo       = "https://github.com/example/skills"
  source_path       = "skills/bootstrap"
  source_ref        = "main"
}
```

## Argument Reference

The following arguments are supported:

* `organization_name` - (Required) The name of the organization this skill belongs to. Changing this forces a new resource.
* `name` - (Required) Slug identifier used in code (e.g. `review`, `bootstrap`). Must be unique per organization. Changing this forces a new resource.
* `display_name` - (Required) Human-readable name for the skill.
* `description` - (Optional) A description of what the skill does.
* `enabled` - (Optional) Whether the skill is enabled. Defaults to `true`.
* `config` - (Optional) JSON string of runtime configuration overrides (model, temperature, system_prompt_prefix, etc.). Use `jsonencode()` for convenience.
* `source_repo` - (Optional) GitHub repository URL for user-defined skills.
* `source_path` - (Optional) Path to the skill entrypoint within the repository.
* `source_ref` - (Optional) Branch, tag, or commit SHA to use from the repository.

## Attributes Reference

In addition to all arguments above, the following attributes are exported:

* `id` - The skill UUID.
* `is_github_sourced` - `true` when `source_repo` is set.
* `created_at` - Timestamp when the skill was created (RFC3339 format).

## Import

Agent skills can be imported using the `organization_name` and `skill_id` separated by a colon:

```
$ terraform import infradots_agent_skill.review my-org:a1b2c3d4-e5f6-7890-abcd-ef1234567890
```
