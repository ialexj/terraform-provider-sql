package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/ialexj/terraform-provider-sql/internal/server"
)

type dataQuery struct {
	db dbConnector
}

var _ server.DataSource = (*dataQuery)(nil)

func newDataQuery(db dbConnector) (*dataQuery, error) {
	if db == nil {
		return nil, fmt.Errorf("a database is required")
	}

	return &dataQuery{
		db: db,
	}, nil
}

// TODO: remove this once its not needed by testing
func deprecatedIDAttribute() *tfprotov6.SchemaAttribute {
	return &tfprotov6.SchemaAttribute{
		Name:       "id",
		Computed:   true,
		Deprecated: true,
		Description: "This attribute is only present for some compatibility issues and should not be used. It " +
			"will be removed in a future version.",
		DescriptionKind: tfprotov6.StringKindMarkdown,
		Type:            tftypes.String,
	}
}

func (d *dataQuery) Schema(context.Context) *tfprotov6.Schema {
	return &tfprotov6.Schema{
		Block: &tfprotov6.SchemaBlock{
			Description:     "The `sql_query` datasource allows you to execute a SQL query against the database of your choice.",
			DescriptionKind: tfprotov6.StringKindMarkdown,
			Attributes: []*tfprotov6.SchemaAttribute{
				{
					Name:     "url",
					Optional: true,
					Computed: true,
					Description: "Database connection strings are specified via URLs. The URL format is driver dependent " +
						"but generally has the form: `dbdriver://username:password@host:port/dbname?param1=true&param2=false`. " +
						"You can optionally set the `SQL_URL` environment variable instead.",
					DescriptionKind: tfprotov6.StringKindMarkdown,
					Type:            tftypes.String,
				},
				{
					Name:            "query",
					Required:        true,
					Description:     "The query to execute. The types in this query will be reflected in the typing of the `result` attribute.",
					DescriptionKind: tfprotov6.StringKindMarkdown,
					Type:            tftypes.String,
				},
				// {
				// 	Name:            "parameters",
				// 	Optional:        true,
				// 	DescriptionKind: tfprotov6.StringKindMarkdown,
				// 	Type:            tftypes.DynamicPseudoType,
				// },

				{
					Name:     "result",
					Computed: true,
					Description: "The result of the query. This will be a list of objects. Each object will have attributes " +
						"with names that match column names and types that match column types. The exact translation of types " +
						"is dependent upon the database driver.",
					DescriptionKind: tfprotov6.StringKindMarkdown,
					Type: tftypes.List{
						ElementType: tftypes.DynamicPseudoType,
					},
				},

				deprecatedIDAttribute(),
			},
		},
	}
}

func (d *dataQuery) Validate(ctx context.Context, config map[string]tftypes.Value) ([]*tfprotov6.Diagnostic, error) {
	// TODO: if connected to server, validate query against it?
	return nil, nil
}

func (d *dataQuery) Read(ctx context.Context, config map[string]tftypes.Value) (map[string]tftypes.Value, []*tfprotov6.Diagnostic, error) {
	var (
		query string
	)

	err := config["query"].As(&query)
	if err != nil {
		return nil, nil, err
	}

	ds, queryer, err := d.db.GetQueryer(ctx, config["url"])
	if err != nil {
		return nil, nil, err
	}

	rows, err := queryer.QueryContext(ctx, query)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	var rowType tftypes.Type
	rowSet := []tftypes.Value{}
	for rows.Next() {
		row, ty, err := ValuesForRow(ds.driver, rows)
		if err != nil {
			return nil, []*tfprotov6.Diagnostic{
				{
					Severity: tfprotov6.DiagnosticSeverityError,
					Attribute: tftypes.NewAttributePathWithSteps([]tftypes.AttributePathStep{
						tftypes.AttributeName("result"),
					}),
					Summary: fmt.Sprintf("unable to convert value from database: %s", err),
				},
			}, nil
		}

		if rowType == nil {
			rowType = tftypes.Object{
				AttributeTypes: ty,
			}
		}

		rowSet = append(rowSet, tftypes.NewValue(
			rowType,
			row,
		))
	}
	if rowType == nil {
		// empty object here
		rowType = tftypes.Object{}
	}

	return map[string]tftypes.Value{
		"id":    config["query"],
		"query": config["query"],
		"url":   config["url"],
		// "parameters": config["parameters"],
		"result": tftypes.NewValue(
			tftypes.List{
				ElementType: rowType,
			},
			rowSet,
		),
	}, nil, nil
}
