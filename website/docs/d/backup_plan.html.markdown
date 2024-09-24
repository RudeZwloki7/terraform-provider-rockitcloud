---
subcategory: "Backup"
layout: "aws"
page_title: "aws_backup_plan"
description: |-
  Provides information about a backup plan.
---

# Data Source: aws_backup_plan

Provides information about a backup plan.

## Example Usage

```terraform
data "aws_backup_plan" "example" {
  plan_id = "01234567-0123-0123-0123-0123456789ab"
}
```

## Argument Reference

The following arguments are supported:

* `plan_id` - (Required) The backup plan ID.

## Attribute Reference

In addition to the arguments above, the following attributes are exported:

* `arn` - The Amazon Resource Name (ARN) of the backup plan.
* `name` - The name of the backup plan.

->  **Unsupported attributes**
These attributes are currently unsupported:

* `version` - Unique, randomly generated, Unicode, UTF-8 encoded string that serves as the version ID of the backup plan. Always `""`.
