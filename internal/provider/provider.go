package provider

import (
	"context"
	"fmt"
	"math/big"
	"os"

	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/ialexj/terraform-provider-sql/internal/server"
)

func New(version string) func() tfprotov6.ProviderServer {
	return func() tfprotov6.ProviderServer {
		s := server.MustNew(func() server.Provider {
			return &provider{}
		})

		// data sources
		s.MustRegisterDataSource("sql_driver", newDataDriver)
		s.MustRegisterDataSource("sql_query", newDataQuery)

		// resources
		s.MustRegisterResource("sql_migrate", newResourceMigrate)
		s.MustRegisterResource("sql_migrate_directory", newResourceMigrateDirectory)

		return s
	}
}

// TODO: use consts for driver names?
type driverName string

type provider struct {
	Url tftypes.Value

	MaxOpenConns int64
	MaxIdleConns int64
}

var _ server.Provider = (*provider)(nil)

func (p *provider) Schema(context.Context) *tfprotov6.Schema {
	return &tfprotov6.Schema{
		Block: &tfprotov6.SchemaBlock{
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
					Name:     "max_open_conns",
					Optional: true,
					Description: "Sets the maximum number of open connections to the database. Default is `0` (unlimited). " +
						"See Go's documentation on [DB.SetMaxOpenConns](https://golang.org/pkg/database/sql/#DB.SetMaxOpenConns).",
					DescriptionKind: tfprotov6.StringKindMarkdown,
					Type:            tftypes.Number,
				},
				{
					Name:     "max_idle_conns",
					Optional: true,
					Description: "Sets the maximum number of connections in the idle connection pool. Default is `2`. " +
						"See Go's documentation on [DB.SetMaxIdleConns](https://golang.org/pkg/database/sql/#DB.SetMaxIdleConns).",
					DescriptionKind: tfprotov6.StringKindMarkdown,
					Type:            tftypes.Number,
				},
			},
		},
	}
}

func (p *provider) Validate(ctx context.Context, config map[string]tftypes.Value) ([]*tfprotov6.Diagnostic, error) {
	return nil, nil
}

func (p *provider) Configure(ctx context.Context, config map[string]tftypes.Value) ([]*tfprotov6.Diagnostic, error) {
	var err error

	url := config["url"]
	if url.IsNull() {
		if env := os.Getenv("SQL_URL"); env != "" {
			url = tftypes.NewValue(tftypes.String, env)
		}
	}

	if url.IsKnown() && !url.IsNull() {
		var urlValue string
		url.As(&urlValue)

		_, err = parseUrl(urlValue)
		if err != nil {
			return nil, fmt.Errorf("ConfigureProvider - invalid url: %w", err)
		}
	}

	p.Url = url

	if v := config["max_open_conns"]; v.IsNull() {
		p.MaxOpenConns = 0
	} else {
		maxOpenConnsBig := &big.Float{}
		err = v.As(&maxOpenConnsBig)
		if err != nil {
			// TODO: diag with path
			return nil, fmt.Errorf("ConfigureProvider - unable to read max_open_conns: %w", err)
		}

		maxOpenConns, acc := maxOpenConnsBig.Int64()
		if acc != big.Exact {
			return nil, fmt.Errorf("ConfigureProvider - max_open_conns must be an integer")
		}

		p.MaxOpenConns = maxOpenConns
	}

	if v := config["max_idle_conns"]; v.IsNull() {
		p.MaxIdleConns = 2
	} else {
		maxIdleConnsBig := &big.Float{}
		err = v.As(&maxIdleConnsBig)
		if err != nil {
			// TODO: diag with path
			return nil, fmt.Errorf("ConfigureProvider - unable to read max_idle_conns: %w", err)
		}

		maxIdleConns, acc := maxIdleConnsBig.Int64()
		if acc != big.Exact {
			return nil, fmt.Errorf("ConfigureProvider - max_idle_conns must be an integer")
		}

		p.MaxIdleConns = maxIdleConns
	}

	return nil, nil
}
