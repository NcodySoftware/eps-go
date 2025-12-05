package epsgo

import (
	"context"
	"embed"
	"os"

	"ncody.com/ncgo.git/database/sql"
	"ncody.com/ncgo.git/database/sql/migrator"
	"ncody.com/ncgo.git/database/sql/sqlite"
)

//go:embed migrations
var migrations embed.FS

func must(err error) {
	if err == nil {
		return
	}
	panic(err)
}

func MustOpenDB(ctx context.Context) sql.Database {
	dirs, err := getXdgDirs(appName)
	must(err)
	dbFilePath := dirs.XDGDataHome+"/db.sqlite3"
	var m migrator.Migrations
	must(m.LoadFromEmbedFS(migrations, "migrations"))
	var flag uint64
	flag = migrator.FlagMigrateAllowUpgrade
	if os.Getenv("MIGRATE_FRESH") == "1" {
		flag |= migrator.FlagMigrateFresh
	}
	mg := migrator.NewSqlite(dbFilePath, m, flag)
	_, err = mg.Migrate(ctx)
	must(err)
	db, err := sqlite.New(dbFilePath)
	must(err)
	return db
}
