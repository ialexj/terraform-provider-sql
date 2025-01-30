package provider

import (
	"context"
	"database/sql"
	"fmt"
	"reflect"
	"strings"
	"time"

	// database drivers

	_ "github.com/denisenkom/go-mssqldb/azuread"
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v4/stdlib"

	// TODO: sqlite? need to use a pure go driver, i think this one is...
	// _ "modernc.org/sqlite"

	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

type dbQueryer interface {
	QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
}

type dbExecer interface {
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
}

type dbConnector interface {
	HasUrl() bool
	GetDataSource() (dataSource, error)
	GetQueryer(ctx context.Context) (dataSource, dbQueryer, error)
	GetExecer(ctx context.Context) (dataSource, dbExecer, error)
}

type dataSource struct {
	driver driverName
	url    string
}

func (p *provider) HasUrl() bool {
	return p.Url.IsKnown()
}

func (p *provider) GetDataSource() (dataSource, error) {
	url, err := p.getCurrentUrlValue()
	if err != nil {
		return dataSource{}, fmt.Errorf("GetDriver - invalid url: %w", err)
	}

	p.DataSource, err = parseUrl(url)
	return p.DataSource, err
}

func (p *provider) getCurrentUrlValue() (string, error) {
	var url string
	err := p.Url.As(&url)
	if err != nil {
		// TODO: diag with path
		return "", fmt.Errorf("unable to read url: %w", err)
	}

	if url == "" {
		return "", fmt.Errorf("url can't be empty")
	}

	return url, nil
}

func (p *provider) GetQueryer(ctx context.Context) (dataSource, dbQueryer, error) {
	return p.connectContext(ctx)
}

func (p *provider) GetExecer(ctx context.Context) (dataSource, dbExecer, error) {
	return p.connectContext(ctx)
}

func (p *provider) connectContext(ctx context.Context) (dataSource, *sql.DB, error) {
	var err error

	if p.DB != nil {
		tflog.Debug(ctx, "Database connection is already open.")
		return p.DataSource, p.DB, nil
	}

	p.DataSource, err = p.GetDataSource()
	if err != nil {
		return p.DataSource, nil, err
	}

	p.DB, err = sql.Open(string(p.DataSource.driver), p.DataSource.url)
	if err != nil {
		return p.DataSource, nil, fmt.Errorf("unable to open database: %w", err)
	}

	p.DB.SetMaxOpenConns(int(p.MaxOpenConns))
	p.DB.SetMaxIdleConns(int(p.MaxIdleConns))

	err = p.DB.PingContext(ctx)
	if err != nil {
		p.DB = nil
		return p.DataSource, nil, fmt.Errorf("ConnectLazy - unable to ping database: %w", err)
	}

	tflog.SetField(ctx, "db_driver", p.DataSource)
	tflog.Info(ctx, "Database connection established.")

	return p.DataSource, p.DB, nil
}

func parseUrl(url string) (dataSource, error) {
	scheme, err := schemeFromURL(url)
	if err != nil {
		return dataSource{}, err
	}

	switch scheme {
	case "postgres", "postgresql":
		// TODO: use consts for these driver names?
		return dataSource{driver: "pgx", url: url}, nil

	case "mysql":
		return dataSource{driver: "mysql", url: strings.TrimPrefix(url, "mysql://")}, nil

		// TODO: multistatements? see go-migrate's implementation
		// https://github.com/golang-migrate/migrate/blob/master/database/mysql/mysql.go
		// TODO: also set parseTime=true https://github.com/go-sql-driver/mysql#parsetime

	case "azuresql":
		return dataSource{driver: "azuresql", url: strings.Replace(url, "azuresql://", "sqlserver://", 1)}, nil

	case "sqlserver":
		return dataSource{driver: "sqlserver", url: url}, nil

	default:
		return dataSource{}, fmt.Errorf("unexpected scheme: %q", scheme)
	}
}

func schemeFromURL(url string) (string, error) {
	if url == "" {
		return "", fmt.Errorf("a datasource name is required")
	}

	i := strings.Index(url, ":")

	// No : or : is the first character.
	if i < 1 {
		return "", fmt.Errorf("a scheme for datasource name is required")
	}

	return url[0:i], nil
}

func ValuesForRow(driver driverName, rows *sql.Rows) (map[string]tftypes.Value, map[string]tftypes.Type, error) {
	colTypes, err := rows.ColumnTypes()
	if err != nil {
		return nil, nil, fmt.Errorf("unable to retrieve column type: %w", err)
	}

	pointers := make([]interface{}, len(colTypes))
	row := map[string]struct {
		index int
		ty    tftypes.Type
		val   interface{}
	}{}

	for i, colType := range colTypes {
		name := colType.Name()
		if name == "?column?" {
			name = fmt.Sprintf("column%d", i)
		}

		ty, rty, err := typeAndValueForColType(driver, colType)
		if err != nil {
			return nil, nil, fmt.Errorf("unable to determine type for %q: %w", name, err)
		}

		val := reflect.New(rty)
		pointers[i] = val.Interface()

		row[name] = struct {
			index int
			ty    tftypes.Type
			val   interface{}
		}{i, ty, val.Interface()}
	}

	err = rows.Scan(pointers...)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to scan values: %w", err)
	}

	rowValues := map[string]tftypes.Value{}
	rowTypes := map[string]tftypes.Type{}
	for k, v := range row {
		val := v.val

		// unwrap sql types
		switch tv := val.(type) {
		case *sql.NullInt64:
			if !tv.Valid {
				val = nil
			} else {
				val = &tv.Int64
			}
		case *sql.NullInt32:
			if !tv.Valid {
				val = nil
			} else {
				val = &tv.Int32
			}
		case *sql.NullFloat64:
			if !tv.Valid {
				val = nil
			} else {
				val = &tv.Float64
			}
		case *sql.NullBool:
			if !tv.Valid {
				val = nil
			} else {
				val = &tv.Bool
			}
		case *sql.NullString:
			if !tv.Valid {
				val = nil
			} else {
				val = &tv.String
			}
		case *sql.NullTime:
			if !tv.Valid {
				val = nil
			} else {
				s := tv.Time.UTC().Format(time.RFC3339)
				val = &s
			}
		}

		rowValues[k] = tftypes.NewValue(
			v.ty,
			val,
		)
		rowTypes[k] = v.ty
	}

	return rowValues, rowTypes, nil
}

func typeAndValueForColType(driver driverName, colType *sql.ColumnType) (tftypes.Type, reflect.Type, error) {
	scanType := colType.ScanType()
	kind := scanType.Kind()

	switch driver {
	case "sqlserver":
		switch dbName := colType.DatabaseTypeName(); dbName {
		case "UNIQUEIDENTIFIER":
			return tftypes.String, reflect.TypeOf((*sqlServerUniqueIdentifier)(nil)).Elem(), nil
		case "DECIMAL", "MONEY", "SMALLMONEY":
			// TODO: add diags about converting to numeric?
			return tftypes.String, reflect.TypeOf((*sql.NullString)(nil)).Elem(), nil
		}
	case "mysql":
		switch dbName := colType.DatabaseTypeName(); dbName {
		case "YEAR":
			return tftypes.Number, reflect.TypeOf((*sql.NullInt32)(nil)).Elem(), nil
		case "VARCHAR", "DECIMAL", "TIME", "JSON":
			return tftypes.String, reflect.TypeOf((*sql.NullString)(nil)).Elem(), nil
		case "DATE", "DATETIME":
			return tftypes.String, reflect.TypeOf((*sql.NullTime)(nil)).Elem(), nil
		}
	case "pgx":
		switch dbName := colType.DatabaseTypeName(); dbName {
		// 790 is the oid of money
		case "MONEY", "790":
			// TODO: add diags about converting to numeric?
			return tftypes.String, reflect.TypeOf((*sql.NullString)(nil)).Elem(), nil
		case "TIMESTAMPTZ", "TIMESTAMP", "DATE":
			return tftypes.String, reflect.TypeOf((*sql.NullTime)(nil)).Elem(), nil
		}
	}

	switch scanType {
	case reflect.TypeOf((*sql.NullInt64)(nil)).Elem(),
		reflect.TypeOf((*sql.NullInt32)(nil)).Elem(),
		reflect.TypeOf((*sql.NullFloat64)(nil)).Elem():
		return tftypes.Number, scanType, nil
	case reflect.TypeOf((*sql.NullString)(nil)).Elem():
		return tftypes.String, scanType, nil
	case reflect.TypeOf((*sql.NullBool)(nil)).Elem():
		return tftypes.Bool, scanType, nil
	case reflect.TypeOf((*sql.NullTime)(nil)).Elem():
		return tftypes.String, scanType, nil
	}

	// Force nullable typing for primitives
	switch kind {
	case reflect.String:
		return tftypes.String, reflect.TypeOf((*sql.NullString)(nil)).Elem(), nil
	case reflect.Int64, reflect.Int32, reflect.Int16, reflect.Int8, reflect.Int,
		reflect.Uint32, reflect.Uint16, reflect.Uint8, reflect.Uint:
		return tftypes.Number, reflect.TypeOf((*sql.NullInt64)(nil)).Elem(), nil
	case reflect.Uint64:
		// TODO: uint64 may be a problem in nullint64 if too large?
		return tftypes.Number, reflect.TypeOf((*sql.NullInt64)(nil)).Elem(), nil
	case reflect.Float32, reflect.Float64:
		return tftypes.Number, reflect.TypeOf((*sql.NullFloat64)(nil)).Elem(), nil
	case reflect.Bool:
		return tftypes.Bool, reflect.TypeOf((*sql.NullBool)(nil)).Elem(), nil
	}

	return nil, nil, fmt.Errorf("unexpected type for %q: %q (%s %s)", colType.Name(), colType.DatabaseTypeName(), kind, scanType)
}
