package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/ialexj/terraform-provider-sql/internal/migration"
)

func completeMigrationsAttribute() *tfprotov6.SchemaAttribute {
	return &tfprotov6.SchemaAttribute{
		Name:     "complete_migrations",
		Computed: true,
		Description: "The completed migrations that have been run against your database. This list is used as " +
			"storage to migrate down or as a trigger for downstream dependencies.",
		DescriptionKind: tfprotov6.StringKindMarkdown,
		Type: tftypes.List{
			ElementType: tftypes.Object{
				AttributeTypes: map[string]tftypes.Type{
					"id":   tftypes.String,
					"up":   tftypes.String,
					"down": tftypes.String,
				},
			},
		},
	}
}

type resourceMigrateCommon struct {
	db dbConnector
}

func (r *resourceMigrateCommon) Read(ctx context.Context, current map[string]tftypes.Value) (map[string]tftypes.Value, []*tfprotov6.Diagnostic, error) {
	// roundtrip current state as the source of applied migrations
	return current, nil, nil
}

func (r *resourceMigrateCommon) Create(ctx context.Context, planned map[string]tftypes.Value, config map[string]tftypes.Value, prior map[string]tftypes.Value) (map[string]tftypes.Value, []*tfprotov6.Diagnostic, error) {
	_, execer, err := r.db.GetExecer(ctx, config["url"])
	if err != nil {
		return nil, nil, err
	}

	plannedMigrations, err := migration.FromListValue(planned["complete_migrations"])
	if err != nil {
		return nil, nil, err
	}

	err = migration.Up(ctx, execer, plannedMigrations, nil)
	if err != nil {
		return nil, nil, err
	}

	return planned, nil, nil
}

func (r *resourceMigrateCommon) Update(ctx context.Context, planned map[string]tftypes.Value, config map[string]tftypes.Value, prior map[string]tftypes.Value) (map[string]tftypes.Value, []*tfprotov6.Diagnostic, error) {
	_, execer, err := r.db.GetExecer(ctx, config["url"])
	if err != nil {
		return nil, nil, err
	}

	priorCompleteMigrations, err := migration.FromListValue(prior["complete_migrations"])
	if err != nil {
		return nil, nil, err
	}

	plannedMigrations, err := migration.FromListValue(planned["complete_migrations"])
	if err != nil {
		return nil, nil, err
	}

	err = migration.Up(ctx, execer, plannedMigrations, priorCompleteMigrations)
	if err != nil {
		return nil, nil, err
	}

	return planned, nil, nil
}

func (r *resourceMigrateCommon) Destroy(ctx context.Context, prior map[string]tftypes.Value) ([]*tfprotov6.Diagnostic, error) {
	_, execer, err := r.db.GetExecer(ctx, prior["url"])
	if err != nil {
		return nil, err
	}

	priorCompleteMigrations, err := migration.FromListValue(prior["complete_migrations"])
	if err != nil {
		return nil, err
	}

	err = migration.Down(ctx, execer, nil, priorCompleteMigrations)
	if err != nil {
		return nil, err
	}

	return nil, nil
}
