# Policy Resource

A plan policy: a rule InfraDots checks against every plan of the workspaces it applies to. An advisory policy reports
its violations in the run's `policy` stage. A blocking one also fails the run before apply.

## Example Usage

```hcl
resource "infradots_policy" "owner_tag" {
  organization_name = "acme"
  name              = "Production must be tagged"
  kind              = "require_tags"
  required_tags     = ["owner", "cost-center"]
  enforcement       = "blocking"
  workspace_tags    = { env = "prod" }
}

resource "infradots_policy" "no_public_buckets" {
  organization_name = "acme"
  name              = "No public bucket ACLs"
  kind              = "attribute"
  resource_type     = "aws_s3_bucket_acl"
  path              = "acl"
  op                = "not_in"
  values            = ["public-read", "public-read-write"]
}

resource "infradots_policy" "db_retention" {
  organization_name = "acme"
  name              = "Seven days of database backups"
  kind              = "attribute"
  resource_type     = "aws_db_instance"
  path              = "backup_retention_period"
  op                = "eq"
  value             = 7
}

resource "infradots_policy" "no_open_ssh" {
  organization_name = "acme"
  name              = "No SSH or RDP from anywhere"
  kind              = "no_open_ingress"
  ports             = [22, 3389]
  enforcement       = "blocking"
}

resource "infradots_policy" "no_delete" {
  organization_name = "acme"
  name              = "Nothing is destroyed in production"
  kind              = "no_delete"
  workspace_tags    = { env = "prod" }
}
```

## Argument Reference

* `organization_name` - (Required) The organization the policy belongs to. Changing it replaces the policy.
* `name` - (Required) The policy's name, unique in the organization. Violations are reported under it.
* `kind` - (Required) What the policy checks: `attribute`, `require_tags`, `no_open_ingress` or `no_delete`.
  Changing it replaces the policy.
* `description` - (Optional) What the policy is for.
* `enforcement` - (Optional) `off`, `advisory` or `blocking`. Defaults to `advisory`.
* `workspace_tags` - (Optional) The policy applies to workspaces carrying every one of these tags. Empty, the
  default, means every workspace.

Each kind reads its own settings, and setting another kind's is an error.

`attribute`:

* `resource_type` - (Required) The resource type, such as `aws_s3_bucket`, or `*` for every resource that has the
  attribute.
* `path` - (Required) The dotted attribute path, such as `acl` or `tags.owner`.
* `op` - (Required) `eq`, `in`, `not_in` or `exists`.
* `value` - (Required with `op = "eq"`) The string, number or bool the attribute must equal. The type counts: `7`
  doesn't match `"7"`.
* `values` - (Required with `op = "in"` or `"not_in"`) The strings the attribute must, or must not, be one of.

`require_tags`:

* `required_tags` - (Required) The tag keys every taggable resource must carry. Tags from the AWS provider's
  `default_tags` count.

`no_open_ingress`:

* `ports` - (Required) The ports an `aws_security_group` must not open to `0.0.0.0/0`.

`no_delete` has no settings. It flags every resource the plan deletes, replacements included.

## Attributes Reference

* `id` - The policy's ID.

## Import

```shell
terraform import infradots_policy.owner_tag acme:<id>
```
