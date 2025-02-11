package provider

import (
	"fmt"
	"testing"

	helperresource "github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

func TestResourceMigrate(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping long test")
	}

	for _, server := range testServers {
		t.Run(server.ServerType, func(t *testing.T) {
			url, _, err := server.URL()
			if err != nil {
				t.Fatal(err)
			}

			// URL is in provider
			helperresource.UnitTest(t, helperresource.TestCase{
				ProtoV6ProviderFactories: protoV6ProviderFactories,
				Steps: []helperresource.TestStep{
					{
						Config: fmt.Sprintf(`
provider "sql" {
	url = %q
	max_idle_conns = 0
}

resource "sql_migrate" "db" {
	migration {
		id   = "create table"
		up   = "CREATE TABLE inline_migrate_test (id integer unique)"
		down = "DROP TABLE inline_migrate_test"
	}
}

data "sql_query" "users" {
	query = "select * from inline_migrate_test"
	depends_on = [sql_migrate.db]
}

output "rowcount" {
	value = length(data.sql_query.users.result)
}
				`, url),
						Check: helperresource.ComposeTestCheckFunc(
							helperresource.TestCheckOutput("rowcount", "0"),
						),
					},
					{
						Config: fmt.Sprintf(`
provider "sql" {
	url = %q
	max_idle_conns = 0
}

resource "sql_migrate" "db" {
	migration {
		id   = "create table"
		up   = "CREATE TABLE inline_migrate_test (id integer unique)"
		down = "DROP TABLE inline_migrate_test"
	}
	migration {
		id   = "insert row"
		up   = "INSERT INTO inline_migrate_test VALUES (1)"
		down = "DELETE FROM inline_migrate_test WHERE id = 1"
	}
}

data "sql_query" "users" {
	query = "select * from inline_migrate_test"
	depends_on = [sql_migrate.db]
}

output "rowcount" {
	value = length(data.sql_query.users.result)
}
				`, url),
						Check: helperresource.ComposeTestCheckFunc(
							helperresource.TestCheckOutput("rowcount", "1"),
						),
					},
				},
			})

			// URL is in resources
			helperresource.UnitTest(t, helperresource.TestCase{
				ProtoV6ProviderFactories: protoV6ProviderFactories,
				Steps: []helperresource.TestStep{
					{
						Config: fmt.Sprintf(`
provider "sql" {
	max_idle_conns = 0
}

resource "sql_migrate" "db" {
	url = %q
	migration {
		id   = "create table"
		up   = "CREATE TABLE inline_migrate_test (id integer unique)"
		down = "DROP TABLE inline_migrate_test"
	}
}

data "sql_query" "users" {
	url   = sql_migrate.db.url
	query = "select * from inline_migrate_test"
	depends_on = [sql_migrate.db]
}

output "rowcount" {
	value = length(data.sql_query.users.result)
}
				`, url),
						Check: helperresource.ComposeTestCheckFunc(
							helperresource.TestCheckOutput("rowcount", "0"),
						),
					},

					// Migrate up
					{
						Config: fmt.Sprintf(`
provider "sql" {
	max_idle_conns = 0
}

resource "sql_migrate" "db" {
	url = %q
	migration {
		id   = "create table"
		up   = "CREATE TABLE inline_migrate_test (id integer unique)"
		down = "DROP TABLE inline_migrate_test"
	}

	migration {
		id   = "insert row"
		up   = "INSERT INTO inline_migrate_test VALUES (4);"
		down = "DELETE FROM inline_migrate_test WHERE id = 4;"
	}
}

data "sql_query" "users" {
	url   = sql_migrate.db.url
	query = "select * from inline_migrate_test"
	depends_on = [sql_migrate.db]
}

output "result" {
	value = data.sql_query.users.result
}

output "rowcount" {
	value = length(data.sql_query.users.result)
}
				`, url),
						Check: helperresource.ComposeTestCheckFunc(
							helperresource.TestCheckOutput("rowcount", "1"),
						),
					},

					//					// Migrate sideways - should redo last
					//					{
					//						Config: fmt.Sprintf(`
					//provider "sql" {
					//	max_idle_conns = 0
					//}
					//
					//resource "sql_migrate" "db" {
					//	url = %q
					//	migration {
					//		id   = "create table"
					//		up   = "CREATE TABLE inline_migrate_test (id integer unique)"
					//		down = "DROP TABLE inline_migrate_test"
					//	}
					//
					//	migration {
					//		id   = "insert row"
					//		up   = "INSERT INTO inline_migrate_test VALUES (2);"
					//		down = "DELETE FROM inline_migrate_test WHERE id = 2;"
					//	}
					//}
					//
					//data "sql_query" "users" {
					//	url   = sql_migrate.db.url
					//	query = "select * from inline_migrate_test"
					//	depends_on = [sql_migrate.db]
					//}
					//
					//output "rowcount" {
					//	value = length(data.sql_query.users.result)
					//}
					//				`, url),
					//						Check: helperresource.ComposeTestCheckFunc(
					//							helperresource.TestCheckOutput("rowcount", "1"),
					//						),
					//					},

					// Migrate down
					{
						Config: fmt.Sprintf(`
provider "sql" {
	max_idle_conns = 0
}

resource "sql_migrate" "db" {
	url = %q
	migration {
		id   = "create table"
		up   = "CREATE TABLE inline_migrate_test (id integer unique)"
		down = "DROP TABLE inline_migrate_test"
	}
	// removed
}

data "sql_query" "users" {
	url   = sql_migrate.db.url
	query = "select * from inline_migrate_test"
	depends_on = [sql_migrate.db]
}

output "result" {
	value = data.sql_query.users.result
}

output "rowcount" {
	value = length(data.sql_query.users.result)
}
				`, url),
						Check: helperresource.ComposeTestCheckFunc(
							helperresource.TestCheckOutput("rowcount", "0"),
						),
					},
				},
			})
		})
	}
}
