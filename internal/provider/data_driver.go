package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/ialexj/terraform-provider-sql/internal/server"
)

type dataDriver struct {
	db dbConnector
}

var _ server.DataSource = (*dataDriver)(nil)

func newDataDriver(db dbConnector) (*dataDriver, error) {
	return &dataDriver{
		db: db,
	}, nil
}

func (d *dataDriver) Schema(context.Context) *tfprotov6.Schema {
	return &tfprotov6.Schema{
		Block: &tfprotov6.SchemaBlock{
			Description: "The `sql_driver` datasource allows you to determine which driver is in use by the provider. This " +
				"is mostly useful for module development when you may communicate with multiple types of databases.",
			DescriptionKind: tfprotov6.StringKindMarkdown,
			Attributes: []*tfprotov6.SchemaAttribute{
				{
					Name:            "name",
					Computed:        true,
					Description:     "The name of the driver, currently this will be one of `pgx`, `mysql`, or `sqlserver`.",
					DescriptionKind: tfprotov6.StringKindMarkdown,
					Type:            tftypes.String,
				},
				{
					Name:            "url",
					Computed:        true,
					Description:     "The URL that's passed to the underlying connection.",
					DescriptionKind: tfprotov6.StringKindMarkdown,
					Type:            tftypes.String,
				},

				deprecatedIDAttribute(),
			},
		},
	}
}

func (d *dataDriver) Validate(ctx context.Context, config map[string]tftypes.Value) ([]*tfprotov6.Diagnostic, error) {
	return nil, nil
}

func (d *dataDriver) Read(ctx context.Context, config map[string]tftypes.Value) (map[string]tftypes.Value, []*tfprotov6.Diagnostic, error) {
	var name, url tftypes.Value

	if !d.db.HasUrl() {
		name = tftypes.NewValue(tftypes.String, tftypes.UnknownValue)
		url = tftypes.NewValue(tftypes.String, tftypes.UnknownValue)
	} else {
		ds, err := d.db.GetDataSource()
		if err != nil {
			return nil, nil, err
		}
		name = tftypes.NewValue(tftypes.String, string(ds.driver))
		url = tftypes.NewValue(tftypes.String, ds.url)
	}

	return map[string]tftypes.Value{
		"name": name,
		"url":  url,

		// just a placeholder, see deprecatedIDAttribute
		"id": tftypes.NewValue(
			tftypes.String,
			"",
		),
	}, nil, nil
}
