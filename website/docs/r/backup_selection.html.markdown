---
subcategory: "Backup"
layout: "aws"
page_title: "aws_backup_selection"
description: |-
  Manages selection conditions for backup plan resources.
---

# Resource: aws_backup_selection

Manages selection conditions for backup plan resources.

## Example Usage

### Selecting Backups By Resource

```terraform
resource "aws_backup_vault_default" "example" {}

data "aws_availability_zones" "this" {
  state = "available"
}

resource "aws_ebs_volume" "example" {
  availability_zone = data.aws_availability_zones.this.names[0]
  type              = "gp2"
  size              = 8
}

resource "aws_backup_plan" "example" {
  name = "tf-backup-plan"

  rule {
    rule_name         = "tf-backup-rule"
    target_vault_name = aws_backup_vault_default.example.name
    schedule          = "cron(0 12 * * *)"
    start_window      = 60
    completion_window = 180

    lifecycle {
      delete_after = 30
    }
  }

  rule {
    rule_name         = "tf-backup-rule2"
    target_vault_name = aws_backup_vault_default.example.name
    schedule          = "cron(0 23 * * *)"
    start_window      = 60
    completion_window = 180

    lifecycle {
      delete_after = 60
    }
  }
}

resource "aws_backup_selection" "example" {
  name    = "tf-backup-selection"
  plan_id = aws_backup_plan.example.id

  resources = [
    aws_ebs_volume.example.arn,
  ]
}
```

## Argument Reference

The following arguments are supported:

* `name` - (Required, Editable) The selection name. The value must be `1` to `50` characters long and must contain only alphanumeric, hyphen, underscore, and period characters.
* `plan_id` - (Required) The backup plan ID.
* `resources` - (Required, Editable) The list of Amazon Resource Names (ARNs) resources included into the backup plan.

## Attribute Reference

In addition to all arguments above, the following attributes are exported:

* `id` - The ID of the resource selection.

->  **Unsupported attributes**
These attributes are currently unsupported:

* `condition` - A list of conditions that you define to assign resources to your backup plans using tags. Always empty.
* `iam_role_arn` -  The Amazon Resource Name (ARN) of the role used for authentication when the resource is backed up (not implemented, added to ensure compatibility with AWS). Always `""`.
* `not_resources` - An array of strings that either contain Amazon Resource Names (ARNs) or match patterns of resources to exclude from a backup plan. Always empty.
* `selection_tag` - Tag-based conditions used to specify a set of resources to assign to a backup plan. Always empty.

## Import

Backup selection can be imported using `plan_id` and `id` separated by `|`.

```
$ terraform import aws_backup_selection.example "01234567-0123-0123-0123-0123456789ab|01234567-0123-0123-0123-0123456789ab"
```
