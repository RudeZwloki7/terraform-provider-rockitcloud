---
subcategory: "PaaS"
page_title: "aws_paas_clickhouse_profile"
description: |-
  Manages a settings profile of a PaaS ClickHouse service.
---

# Resource: aws_paas_clickhouse_profile

Manages a settings profile of a PaaS ClickHouse service.

~> **Note** To assign this profile to a user managed inside the parent `aws_paas_service`,
follow the [staged apply workflow](paas_service.md#clickhouse-user).
The profile must already exist when the user is assigned to it.

~> **Note** A profile assigned to a user cannot be deleted.
Before removing this resource or running `terraform destroy`, change every affected user's `profile`
to an existing alternative such as `default` and apply that change first.

## Example Usage

```terraform
resource "aws_paas_clickhouse_profile" "example" {
  service_id = aws_paas_service.clickhouse.id
  name       = "example_profile"

  settings = {
    max_memory_usage = "10000000000"
    max_threads      = "4"
  }
}
```

## Argument Reference

The following arguments are supported:

* `name` - (Required) The profile name. The value must start with a Latin letter and
  can only contain Latin letters, numbers and underscores (_).
  The value must not be `default` and `readonly`.
* `service_id` - (Required) The ID of the PaaS ClickHouse service.
* `settings` - (Optional, Editable) Map containing ClickHouse settings of the profile. Values are strings.

## Attribute Reference

In addition to all arguments above, the following attributes are exported:

* `id` - The ID of the profile in the format `<service_id>/<profile_id>`.

## Timeouts

The `timeouts` block allows you to specify [timeouts](https://www.terraform.io/docs/configuration/blocks/resources/syntax.html#operation-timeouts) for certain actions:

* `create` - (Default `30 minutes`) How long to wait for the service to be updated after the profile is created.
* `update` - (Default `30 minutes`) How long to wait for the service to be updated after the profile is modified.
* `delete` - (Default `30 minutes`) How long to wait for the service to be updated after the profile is deleted.

## Import

PaaS ClickHouse profile can be imported using `id`, e.g.,

```
$ terraform import aws_paas_clickhouse_profile.example fm-cluster-12345678/clickhouse-profile-12345678
```
