package provider

import (
	"fmt"
	"strings"
	"testing"

	helper "github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

func testStep(url string, urlInProvider bool, migrations []string, expectedRows string) helper.TestStep {
	urlParameter := fmt.Sprintf(`url = "%s"`, url)

	var providerUrl, resourceUrl string
	if urlInProvider {
		providerUrl = urlParameter
	} else {
		resourceUrl = urlParameter
	}

	return helper.TestStep{
		Config: fmt.Sprintf(
			`provider "sql" {
				max_idle_conns = 0
				%s
			}
			resource "sql_migrate" "db" {
				%s
				%s
			}
			data "sql_query" "users" {
				%s
				query = "select * from test"
				depends_on = [sql_migrate.db]
			}
			output "rowcount" {
				value = length(data.sql_query.users.result)
			}
			`, providerUrl, resourceUrl, strings.Join(migrations, "\n"), resourceUrl),

		Check: helper.ComposeTestCheckFunc(
			helper.TestCheckOutput("rowcount", expectedRows),
		),
	}
}

func testSet(url string, urlInProvider bool) []helper.TestStep {
	migration_create_table := `migration {
		id   = "create table"
		up   = "CREATE TABLE test (id integer unique)"
		down = "DROP TABLE test"
	}`

	migration_insert_row := `migration {
		id   = "insert row"
		up   = "INSERT INTO test VALUES (1)"
		down = "DELETE FROM test WHERE id = 1"
	}`

	return []helper.TestStep{
		testStep(url, urlInProvider, []string{
			migration_create_table, // create table
		}, "0"),
		testStep(url, urlInProvider, []string{
			migration_create_table,
			migration_insert_row, // add a row
		}, "1"),
		testStep(url, urlInProvider, []string{
			migration_create_table,
			// delete row
		}, "0"),
	}
}

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

			helper.UnitTest(t, helper.TestCase{
				ProtoV6ProviderFactories: protoV6ProviderFactories,
				Steps: append(
					testSet(url, true),     // test with URL in provider
					testSet(url, false)..., // test with URL in resource
				),
			})
		})
	}
}
