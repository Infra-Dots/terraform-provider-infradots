# Agent Skill Resource

An organization's agent skill. Agents see each skill's one-line `description` in their skill index and load its
instructions only when the skill fits the run. A skill is either inline (`instructions`) or a skill directory in
git (`source_repo` + `source_path`).

Platform skills reach every organization. A skill whose `name` is a platform skill's replaces it for the
organization; one with `enabled = false` and no content turns it off. Organization skills pass the same input
check as implement requests when they are saved: a refused skill fails the apply with the reason.

Creating, changing or deleting a skill needs the `write_agents` permission.

## Example Usage

```hcl
# Inline: the SKILL.md body, without its frontmatter
resource "infradots_agent_skill" "tagging" {
  organization_name = "my-org"
  name              = "tagging"
  display_name      = "Tagging"
  description       = "Required tags for every resource. Use when adding or changing resources."
  instructions      = file("${path.module}/skills/tagging.md")
  resource_types    = ["aws_*"]
}

# From git, pinned to the commit source_ref points to when it is saved
resource "infradots_agent_skill" "iac_security_review" {
  organization_name = "my-org"
  name              = "iac-security-review"
  display_name      = "IaC security review"
  source_repo       = "https://github.com/atkozhuharov/iac-ai"
  source_path       = "skills/iac-security-review"
  source_ref        = "main"
  agents            = ["review"]
}

# Turn a platform skill off for the organization
resource "infradots_agent_skill" "no_scaffold" {
  organization_name = "my-org"
  name              = "iac-module-scaffold"
  display_name      = "Module scaffold (off)"
  enabled           = false
}
```

## Argument Reference

The following arguments are supported:

* `organization_name` - (Required) The name of the organization this skill belongs to. Changing this forces a new resource.
* `name` - (Required) The name agents load the skill by: lowercase letters, digits and single hyphens. Unique per organization; a platform skill's name replaces that skill. Changing this forces a new resource.
* `display_name` - (Required) Human-readable name for the skill.
* `description` - (Optional) One line, at most 128 characters: what the skill does and when to use it. Required for an inline skill; a git skill without one uses its SKILL.md's.
* `instructions` - (Optional) The instructions (Markdown) of an inline skill, at most 100,000 characters.
* `source_repo` - (Optional) `https://` repository holding the skill directory. The organization's VCS token is sent only to the host that issued it.
* `source_path` - (Optional) The skill directory in `source_repo`, holding `SKILL.md`. Its name, and SKILL.md's `name`, must equal `name`.
* `source_ref` - (Optional) Branch, tag or commit of `source_repo`. Saving pins the commit it points to; pin a tag or commit and change it to move the skill.
* `agents` - (Optional) Only these agents list the skill: `review`, `implement`, `remediate`. Empty: all.
* `resource_types` - (Optional) Only runs touching these resource types list the skill; patterns such as `aws_iam_*`. Empty: every run.
* `iac_types` - (Optional) Only `TF`, `OT` and/or `TG` workspaces. Empty: all.
* `enabled` - (Optional) Whether agents use the skill. Defaults to `true`.
* `config` - (Optional) JSON string of runtime configuration. Unused by agents today.

A git skill may declare `agents`, `resource_types` and `iac_types` in its SKILL.md frontmatter `metadata`
(`infradots-agents`, `infradots-resource-types`, `infradots-iac-types`, comma-separated); the resource's values win.

## Attributes Reference

In addition to all arguments above, the following attributes are exported:

* `id` - The skill UUID.
* `source_commit` - The commit `source_ref` pointed to when the skill was saved and screened: what runs load.
* `is_github_sourced` - `true` when `source_repo` is set.
* `created_at` - Timestamp when the skill was created (RFC3339 format).

## Import

Agent skills can be imported using the `organization_name` and `skill_id` separated by a colon:

```
$ terraform import infradots_agent_skill.review my-org:a1b2c3d4-e5f6-7890-abcd-ef1234567890
```
