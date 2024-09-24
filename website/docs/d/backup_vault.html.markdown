---
subcategory: "Backup"
layout: "aws"
page_title: "aws_backup_vault"
description: |-
  Provides information about a backup vault.
---

# Data Source: aws_backup_vault

Provides information about a backup vault.
Use this data source to get information on an existing backup vault.

## Example Usage

```terraform
data "aws_backup_vault" "example" {
  name = "Default"
}
```

## Argument Reference

The following arguments are supported:

* `name` - (Required) The name of the backup vault.

## Attribute Reference

In addition to all arguments above, the following attributes are exported:

* `arn` - The Amazon Resource Name (ARN) of the backup vault.
* `recovery_points` - The number of recovery points in the vault.

->  **Unsupported attributes**
These attributes are currently unsupported:

* `kms_key_arn` - The server-side encryption key that is used to protect your backups. Always `""`.
