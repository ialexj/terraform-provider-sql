package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/ialexj/terraform-provider-sql/internal/migration"
	"github.com/ialexj/terraform-provider-sql/internal/server"
)

type resourceMigrateDirectory struct {
	resourceMigrateCommon
}

var _ server.Resource = (*resourceMigrateDirectory)(nil)
var _ server.ResourceUpdater = (*resourceMigrateDirectory)(nil)

func newResourceMigrateDirectory(db dbConnector) (*resourceMigrateDirectory, error) {
	return &resourceMigrateDirectory{
		resourceMigrateCommon: resourceMigrateCommon{
			db: db,
		},
	}, nil
}

var (
	_ server.Resource        = (*resourceMigrateDirectory)(nil)
	_ server.ResourceUpdater = (*resourceMigrateDirectory)(nil)
)

func (r *resourceMigrateDirectory) Schema(ctx context.Context) *tfprotov6.Schema {
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
					Name:     "path",
					Required: true,
					Description: "The path of the SQL migration files. For a path relative to the current module, " +
						"use `path.module`.",
					DescriptionKind: tfprotov6.StringKindMarkdown,
					Type:            tftypes.String,
				},
				{
					Name:     "single_file_split",
					Optional: true,
					Description: fmt.Sprintf("Set this to a value if your migration up and down are in a single "+
						"file, split on some constant string (ie. in the case of [shmig](https://github.com/mbucc/shmig) "+
						"you would use `%s`).", migration.SHMigSplit),
					DescriptionKind: tfprotov6.StringKindMarkdown,
					Type:            tftypes.String,
				},
				completeMigrationsAttribute(),
				deprecatedIDAttribute(),
			},
		},
	}
}

func (r *resourceMigrateDirectory) Validate(ctx context.Context, config map[string]tftypes.Value) ([]*tfprotov6.Diagnostic, error) {
	return nil, nil
}

func (r *resourceMigrateDirectory) PlanCreate(ctx context.Context, proposed map[string]tftypes.Value, config map[string]tftypes.Value) (map[string]tftypes.Value, []*tfprotov6.Diagnostic, error) {
	return r.plan(ctx, proposed)
}

func (r *resourceMigrateDirectory) PlanUpdate(ctx context.Context, proposed map[string]tftypes.Value, config map[string]tftypes.Value, prior map[string]tftypes.Value) (map[string]tftypes.Value, []*tfprotov6.Diagnostic, error) {
	return r.plan(ctx, proposed)
}

func (r *resourceMigrateDirectory) plan(ctx context.Context, proposed map[string]tftypes.Value) (map[string]tftypes.Value, []*tfprotov6.Diagnostic, error) {
	if !proposed["path"].IsFullyKnown() || !proposed["single_file_split"].IsFullyKnown() {
		return map[string]tftypes.Value{
			"id":                  tftypes.NewValue(tftypes.String, "static-id"),
			"path":                proposed["path"],
			"single_file_split":   proposed["single_file_split"],
			"complete_migrations": tftypes.NewValue(migration.ListTFType, tftypes.UnknownValue),
		}, nil, nil
	}

	var (
		err error

		path            string
		singleFileSplit string
	)

	err = proposed["path"].As(&path)
	if err != nil {
		return nil, nil, err
	}

	err = proposed["single_file_split"].As(&singleFileSplit)
	if err != nil {
		return nil, nil, err
	}

	migrations, err := migration.ReadDir(path, &migration.Options{
		StripLineComments: true,
		SingleFileSplit:   singleFileSplit,
	})
	// TODO: diagnostics here for common file issues, etc?
	if err != nil {
		return nil, nil, err
	}

	return map[string]tftypes.Value{
		"id":                  tftypes.NewValue(tftypes.String, "static-id"),
		"url":                 proposed["url"],
		"path":                proposed["path"],
		"single_file_split":   proposed["single_file_split"],
		"complete_migrations": migration.List(migrations),
	}, nil, nil
}
