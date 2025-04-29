# Terraform SQL Provider

This Terraform provider allows provisioning of SQL schema from Terraform (tables, views, etc.), using an up/down migration style.

View the docs on the [Terraform Registry](https://registry.terraform.io/providers/ialexj/sql/latest/docs).

## Quick Start

```terraform
terraform {
  required_providers {
    sql = {
      source = "ialexj/sql"
      version = "1.0.1"
    }
  }
}

provider "sql" {
  alias = myserver
  # url = "sqlserver://sa:password@localhost:1433"
  # url = "azuresql://myserver.azurewebsites.com?fedauth=ActiveDirectoryAzCli"
  # url = "postgres://postgres:password@localhost:5432/mydatabase?sslmode=disable"
  # url = "postgres://root@localhost:26257/events?sslmode=disable"
  # url = "mysql://root:password@tcp(localhost:3306)/mysql"
}

resource "sql_migrate" "create_users" {
  provider = sql.myserver

  migration {
    up = <<-EOT
      CREATE TABLE users (
          user_id integer unique,
          name    varchar(40),
          email   varchar(40)
      )
      EOT

    down = "DROP TABLE IF EXISTS users"
  }
}

data "sql_query" "users" {
  # run this query after the migration
  depends_on = [sql_migrate.db]

  query = "select * from users"
}

output "rowcount" {
  value = length(data.sql_query.users.result)
}
```

## Fork

This is a fork of the [sql provider by paultyng](https://github.com/paultyng/terraform-provider-sql). The original provider hasn't been updated in a few years, and has several issues that have been fixed in this fork:

- Connection is deferred until apply-time. This makes it possible to use the provider with a yet-unknown URL, such as one for a server that will be deployed in the same apply.
- Support for Azure Auth for SQL Server. This is achieved by swapping in [Microsoft's SQL Server driver](https://github.com/microsoft/go-mssqldb), with `azuread` support.

### Known Issues

There are a few known issues that will be addressed in time.

- When using a single resource to specify many migrations, if one migration in the set succeeds, but the next one fails, the first one's success will not get recorded in the state. On the next apply, all the migrations will be re-run. This can be avoided if using only one migration on each resource, and using `depends_on` to ensure ordering.
- The requirement to specify the URL at the provider level is limiting. It should be possible to specify it at the resource level instead.
- This provider uses the legacy [terraform-plugin-go](https://github.com/hashicorp/terraform-plugin-go) SDK.

## Azure Auth

Azure Auth for SQL Server is enabled by using an `azuresql://` URL.

The driver will **NOT** pick up the identity/token that Terraform's using with `azurerm`. Instead, you must set the `fedauth=...` value in the URL [as explained here](https://github.com/microsoft/go-mssqldb?tab=readme-ov-file#azure-active-directory-authentication).

Credentials can be passed either via the URL, or via specific [environment variables](https://github.com/Azure/azure-sdk-for-go/tree/main/sdk/azidentity#environment-variables).

For passwordless login, either `ActiveDirectoryManagedIdentity` should be used to pick up on the current Managed Identity, or `ActiveDirectoryAzCli` to reuse the currently authenticated `az` CLI session.

It's worth noting that when using `ActiveDirectoryDefault`, Managed Identity will take precedence over Azure CLI, which could be meaningful in the context of a private workflow runner, which itself has a managed identity, but uses Azure CLI to authenticate the workflow.


