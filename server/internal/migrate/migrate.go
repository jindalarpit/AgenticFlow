package migrate

import (
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

// Run applies all pending migrations. It acquires a PostgreSQL advisory lock
// to prevent concurrent runs. Returns an error if any migration fails.
func Run(databaseURL, migrationsPath string) error {
	start := time.Now()

	// golang-migrate's pgx/v5 driver expects the "pgx5://" scheme.
	// Convert standard "postgres://" or "postgresql://" to "pgx5://".
	dbURL := databaseURL
	if strings.HasPrefix(dbURL, "postgres://") {
		dbURL = "pgx5://" + strings.TrimPrefix(dbURL, "postgres://")
	} else if strings.HasPrefix(dbURL, "postgresql://") {
		dbURL = "pgx5://" + strings.TrimPrefix(dbURL, "postgresql://")
	}

	m, err := migrate.New("file://"+migrationsPath, dbURL)
	if err != nil {
		return fmt.Errorf("create migrator: %w", err)
	}
	defer m.Close()

	// golang-migrate uses advisory locks by default with pgx driver.
	err = m.Up()
	if err == migrate.ErrNoChange {
		slog.Info("migrations up to date", "duration", time.Since(start))
		return nil
	}
	if err != nil {
		ver, dirty, _ := m.Version()
		return fmt.Errorf("migration %d (dirty=%v) failed: %w", ver, dirty, err)
	}
	slog.Info("migrations applied", "duration", time.Since(start))
	return nil
}
