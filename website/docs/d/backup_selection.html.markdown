---
subcategory: "Backup"
layout: "aws"
page_title: "aws_backup_selection"
description: |-
  Provides information about a backup selection.
---

# Data source: aws_backup_selection

Provides information about a backup selection.

## Example Usage

```terraform
data "aws_backup_plan" "example" {
  plan_id = "01234567-0123-0123-0123-0123456789ab"
}

data "aws_backup_selection" "example" {
  plan_id      = data.aws_backup_plan.example.id
  selection_id = "01234567-0123-0123-0123-0123456789ab"
}
```

## Argument Reference

The following arguments are supported:

* `plan_id` - (Required) The ID of the backup plan.
* `selection_id` - (Required) The ID of the resource selection.

## Attribute Reference

In addition to all arguments above, the following attributes are exported:

* `name` - The selection name.
* `resources` - The list of Amazon Resource Names (ARNs) of the resources included into the backup plan.

->  **Unsupported attributes**
These attributes are currently unsupported:

* `iam_role_arn` - The Amazon Resource Name (ARN) of the role used for authentication when the resource is backed up. Always `""`.
