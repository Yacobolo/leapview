package materialize

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/Yacobolo/libredash/internal/analytics/duckdb"
	semanticmodel "github.com/Yacobolo/libredash/internal/analytics/model"
)

type MissingDataError struct {
	DataDir string
	Missing []string
}

func (e *MissingDataError) Error() string {
	return fmt.Sprintf("local source files are missing in %s: %s. Run the workspace bootstrap script or set LIBREDASH_DATA_DIR.", e.DataDir, strings.Join(e.Missing, ", "))
}

func (e *MissingDataError) SetupRequired() bool {
	return true
}

func Refresh(ctx context.Context, db *sql.DB, model *semanticmodel.Model, dataDir string, attachedConnections map[string]struct{}) (time.Time, error) {
	if err := duckdb.RegisterSourceViews(ctx, db, model, dataDir, attachedConnections); err != nil {
		return time.Time{}, err
	}
	if err := ModelTables(ctx, db, model); err != nil {
		return time.Time{}, err
	}
	return time.Now(), nil
}

func ValidateFiles(model *semanticmodel.Model, dataDir string) error {
	var missing []string
	for name, source := range model.Sources {
		if source.Path == "" {
			continue
		}
		connection := model.Connections[source.Connection]
		if connection.Kind != "local" {
			continue
		}
		file, err := duckdb.ResolveSourcePath(model, source, dataDir)
		if err != nil {
			return fmt.Errorf("resolving local source %s: %w", name, err)
		}
		if _, err := os.Stat(file); errors.Is(err, os.ErrNotExist) {
			missing = append(missing, file)
		} else if err != nil {
			return err
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return &MissingDataError{DataDir: dataDir, Missing: missing}
	}
	return nil
}

func ModelTables(ctx context.Context, db *sql.DB, model *semanticmodel.Model) error {
	for _, name := range model.TableNames() {
		if err := validateIdentifier(name); err != nil {
			return err
		}
		table := model.Tables[name]
		sourceSQL := table.Transform.SQL
		if table.Source != "" {
			if err := validateIdentifier(table.Source); err != nil {
				return err
			}
			if sourceSQL == "" {
				sourceSQL = "SELECT * FROM raw." + table.Source
			}
		}
		stmt := fmt.Sprintf("CREATE OR REPLACE TABLE model.%s AS %s", name, sourceSQL)
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("materializing model.%s: %w", name, err)
		}
	}
	return nil
}

func validateIdentifier(value string) error {
	for i, r := range value {
		if i == 0 {
			if (r < 'A' || r > 'Z') && (r < 'a' || r > 'z') && r != '_' {
				return fmt.Errorf("invalid identifier %q", value)
			}
			continue
		}
		if (r < 'A' || r > 'Z') && (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '_' {
			return fmt.Errorf("invalid identifier %q", value)
		}
	}
	if value == "" {
		return fmt.Errorf("invalid identifier %q", value)
	}
	return nil
}
