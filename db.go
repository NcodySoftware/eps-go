package epsgo

import (
	"context"
	"embed"

	"ncody.com/ncgo.git/database/sql"
	"ncody.com/ncgo.git/database/sql/migrator"
	"ncody.com/ncgo.git/database/sql/sqlite"
	"ncody.com/ncgo.git/stackerr"
)

//go:embed migrations
var migrations embed.FS

func must(err error) {
	if err == nil {
		return
	}
	panic(err)
}

func MustOpenDB(ctx context.Context, cfg Config) sql.Database {
	dbFilePath := cfg.XDGDirs.XDGDataHome + "/db.sqlite3"
	var flag uint64
	flag = migrator.FlagMigrateAllowUpgrade
	if cfg.MigrateFresh == "1" {
		flag |= migrator.FlagMigrateFresh
	}
	db, err := OpenDB(ctx, dbFilePath, flag)
	must(err)
	return db
}

func OpenDB(
	ctx context.Context, dbFilePath string, migratorFlag uint64,
) (sql.Database, error) {
	var (
		m migrator.Migrations
	)
	err := m.LoadFromEmbedFS(migrations, "migrations")
	if err != nil {
		return nil, stackerr.Wrap(err)
	}
	mg := migrator.NewSqlite(dbFilePath, m, migratorFlag)
	_, err = mg.Migrate(ctx)
	if err != nil {
		return nil, stackerr.Wrap(err)
	}
	db, err := sqlite.New(dbFilePath)
	if err != nil {
		return nil, stackerr.Wrap(err)
	}
	return db, nil
}
