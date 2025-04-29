package provider

import (
	"fmt"
	"strings"
	"testing"

	helper "github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

func testStep(url string, urlInProvider bool, migrations []string, expectedRows string, expectedValue string) helper.TestStep {
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
				query = "SELECT id FROM test"
				depends_on = [sql_migrate.db]
			}
			output "rowcount" {
				value = length(data.sql_query.users.result)
			}
			output "value" {
				value = try(data.sql_query.users.result[0].id, "")
			}
			`, providerUrl, resourceUrl, strings.Join(migrations, "\n"), resourceUrl),

		Check: helper.ComposeTestCheckFunc(
			helper.TestCheckOutput("rowcount", expectedRows),
			helper.TestCheckOutput("value", expectedValue),
		),
	}
}

func testSet(url string, urlInProvider bool) []helper.TestStep {
	migration_create_table := `migration {
		id   = "create table"
		up   = "CREATE TABLE test (id integer unique)"
		down = "DROP TABLE test"
	}`

	migration_insert_row := func(i int) string {
		return fmt.Sprintf(`migration {
			id   = "insert row %d"
			up   = "INSERT INTO test VALUES (%d)"
			down = "DELETE FROM test WHERE id = %d"
		}`, i, i, i)
	}

	return []helper.TestStep{
		testStep(url, urlInProvider, []string{
			migration_create_table, // create table
		}, "0", ""),
		testStep(url, urlInProvider, []string{
			migration_create_table,
			migration_insert_row(1), // add a row
		}, "1", "1"),
		// This actually fails
		testStep(url, urlInProvider, []string{
			migration_create_table,
			migration_insert_row(2), // delete row 1, add row 2
		}, "1", "2"),
		testStep(url, urlInProvider, []string{
			migration_create_table,
			// delete row
		}, "0", ""),
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
