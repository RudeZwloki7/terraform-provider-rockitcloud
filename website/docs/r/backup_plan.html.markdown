---
subcategory: "Backup"
layout: "aws"
page_title: "aws_backup_plan"
description: |-
  Manages a backup plan.
---

[backup-plan]: https://docs.cloud.croc.ru/en/services/backup/operations.html#backupplan

# Resource: aws_backup_plan

Manages a backup plan. For details about backup plans, see the [user documentation][backup-plan].

## Example Usage

```terraform
resource "aws_backup_vault_default" "example" {}

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
```

## Argument Reference

The following arguments are supported:

* `name` - (Required, Editable) The name of the backup plan.
* `rule` - (Required, Editable) List of rule objects for separate periodic tasks. The structure of this block is [described below](#rule)

### Rule

~> All the parameters in the `rule` block are editable.

The `rule` block has the following structure:

* `completion_window` - (Optional) Time in minutes after the start of the backup job, during which it should be completed. Otherwise, it will be canceled. Defaults to `180`.
* `lifecycle` - (Optional) The lifecycle defines when a protected resource is transitioned to cold storage and when it expires. The structure of the block is [described below](#lifecycle).
* `rule_name` - (Required) The name of the backup rule. The value must be `1` to `50` characters long and must contain only alphanumeric characters, hyphens, underscores, and periods.
* `schedule` - (Optional) CRON expression in UTC for the backup job scheduling.
* `start_window` - (Optional) Time in minutes during which a backup job should start running. Otherwise the job will be cancelled. Defaults to `60`.
* `target_vault_name` - (Required) The name of the backup vault. The value must be `2` to `50` characters long and must contain only alphanumeric characters, hyphens, and underscores.

### Lifecycle

The `lifecycle` block has the following structure:

* `delete_after` - (Optional, Editable) Number of days from the recovery point creation date, after which it will be deleted.

## Attribute Reference

In addition to all arguments above, the following attributes are exported:

* `arn` - The Amazon Resource Name (ARN) of the backup plan.
* `id` - The backup plan ID.

->  **Unsupported attributes**
These attributes are currently unsupported:

* `advanced_backup_setting` - An object that specifies backup options for each resource type. Always empty.
* `rule` - A rule object that specifies a separate periodic task.
    * `copy_action` - Configuration block(s) with copy operation settings.
    * `enable_continuous_backup` - Enable continuous backups for supported resources. Defaults to `false`.
    * `lifecycle` - The lifecycle defines when a protected resource is transitioned to cold storage and when it expires.
        * `cold_storage_after` - Specifies the number of days after creation that a recovery point is moved to cold storage.
    * `recovery_point_tags` - Metadata that you can assign to help organize the resources that you create.
* `version` - Unique, randomly generated, Unicode, UTF-8 encoded string that serves as the version ID of the backup plan. Always `""`.

## Import

Backup Plan can be imported using `id`, e.g.,

```
$ terraform import aws_backup_plan.example 01234567-0123-0123-0123-0123456789ab
```
