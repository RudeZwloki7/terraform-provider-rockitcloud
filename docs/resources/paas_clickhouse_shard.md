---
subcategory: "PaaS"
layout: "aws"
page_title: "aws_paas_clickhouse_shard"
description: |-
  Manages a shard of a PaaS ClickHouse service.
---

# Resource: aws_paas_clickhouse_shard

Manages a shard of a PaaS ClickHouse service.

~> **Note** Shards cannot be removed from the service. Terraform does not delete the shard when running `terraform destroy`,
it will only be removed from the Terraform state.

~> **Note** A standalone service supports only one shard with one replica.
The total number of replicas of all shards must not exceed the number of server instances of the service.

## Example Usage

```terraform
resource "aws_paas_clickhouse_shard" "example" {
  service_id = aws_paas_service.clickhouse.id

  internal_replication = true
  replicas             = 3
  weight               = 1
}
```

## Argument Reference

The following arguments are supported:

* `internal_replication` - (Optional, Editable) Indicates whether data is written to only one replica of the shard
  and replicated by ClickHouse.
    * _Default value:_ `false`
* `replicas` - (Optional, Editable) The number of shard replicas. The value cannot be decreased.
    * _Valid values:_ Greater or equal to `1`
    * _Default value:_ `1`
* `service_id` - (Required) The ID of the PaaS ClickHouse service.
* `weight` - (Optional, Editable) The shard weight used to distribute data between shards.
    * _Valid values:_ From 1 to 2147483647
    * _Default value:_ `1`

## Attribute Reference

In addition to all arguments above, the following attributes are exported:

* `id` - The ID of the shard in the format `<service_id>/<index>`.
* `index` - The index of the shard in the service.

## Timeouts

The `timeouts` block allows you to specify [timeouts](https://www.terraform.io/docs/configuration/blocks/resources/syntax.html#operation-timeouts) for certain actions:

* `create` - (Default `30 minutes`) How long to wait for the service to be updated after the shard is added.
* `update` - (Default `30 minutes`) How long to wait for the service to be updated after the shard is modified.

## Import

PaaS ClickHouse shard can be imported using `id`, e.g.,

```
$ terraform import aws_paas_clickhouse_shard.example fm-cluster-12345678/0
```
