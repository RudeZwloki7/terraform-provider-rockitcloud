---
subcategory: "PaaS"
page_title: "aws_paas_clickhouse_quota"
description: |-
  Manages a quota of a PaaS ClickHouse service.
---

# Resource: aws_paas_clickhouse_quota

Manages a quota of a PaaS ClickHouse service.

~> **Note** To assign this quota to a user managed inside the parent `aws_paas_service`,
follow the [staged apply workflow](paas_service.md#clickhouse-user).
The quota must already exist when the user is assigned to it.

~> **Note** A quota assigned to a user cannot be deleted.
Before removing this resource or running `terraform destroy`, change every affected user's `quota`
to an existing alternative such as `default` and apply that change first.

## Example Usage

```terraform
resource "aws_paas_clickhouse_quota" "example" {
  service_id = aws_paas_service.clickhouse.id
  name       = "example_quota"

  interval {
    duration = 3600
    queries  = 1000
    errors   = 100
  }

  interval {
    duration       = 86400
    read_rows      = 1000000000
    execution_time = 3600
  }
}
```

## Argument Reference

The following arguments are supported:

* `interval` - (Required, Editable) List of intervals with limits. The structure of this block is [described below](#interval).
* `name` - (Required) The quota name. The value must start with a Latin letter and
  can only contain Latin letters, numbers and underscores (_).
  The value must not be `default`.
* `service_id` - (Required) The ID of the PaaS ClickHouse service.

### interval

The `interval` block has the following structure:

* `duration` - (Required) The interval duration in seconds.
    * _Valid values:_ From 1 to 2147483647
* `errors` - (Optional) The maximum number of queries that threw an exception.
* `execution_time` - (Optional) The maximum total query execution time in seconds. A floating-point number.
* `failed_sequential_authentications` - (Optional) The maximum number of sequential authentication failures.
* `queries` - (Optional) The maximum total number of queries.
* `query_inserts` - (Optional) The maximum total number of insert queries.
* `query_selects` - (Optional) The maximum total number of select queries.
* `read_bytes` - (Optional) The maximum total number of bytes read from all tables and table functions.
* `read_rows` - (Optional) The maximum total number of rows read from all tables and table functions.
* `result_bytes` - (Optional) The maximum total number of bytes given as a result.
* `result_rows` - (Optional) The maximum total number of rows given as a result.
* `written_bytes` - (Optional) The maximum total number of bytes written.

At least one limit must be set for each interval. All limits must be greater than or equal to `1`.

## Attribute Reference

In addition to all arguments above, the following attributes are exported:

* `id` - The ID of the quota in the format `<service_id>/<quota_id>`.

## Timeouts

The `timeouts` block allows you to specify [timeouts](https://www.terraform.io/docs/configuration/blocks/resources/syntax.html#operation-timeouts) for certain actions:

* `create` - (Default `30 minutes`) How long to wait for the service to be updated after the quota is created.
* `update` - (Default `30 minutes`) How long to wait for the service to be updated after the quota is modified.
* `delete` - (Default `30 minutes`) How long to wait for the service to be updated after the quota is deleted.

## Import

PaaS ClickHouse quota can be imported using `id`, e.g.,

```
$ terraform import aws_paas_clickhouse_quota.example fm-cluster-12345678/clickhouse-quota-12345678
```
