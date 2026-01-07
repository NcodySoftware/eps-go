package migrator

import (
	"bytes"
	"context"
	"crypto/sha256"
	"embed"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"ncody.com/ncgo.git/database/sql"
	"ncody.com/ncgo.git/database/sql/sqlite"
	"ncody.com/ncgo.git/stackerr"
)

const (
	FlagMigrateFresh uint64 = 1 << iota
	FlagMigrateAllowUpgrade
)

const (
	DBKind_Postgres byte = iota
	DBKind_Sqlite
)

var (
	ErrDirtyState = errors.New("migration at dirty state")
)

type Migrator interface {
	Migrate(ctx context.Context) (int64, error)
}

type MigratorPostgres struct {
	db         sql.Database
	schema     string
	flag       uint64
	migrations []migration
}

func NewPostgres(
	db sql.Database, migrations Migrations, schema string, flag uint64,
) Migrator {
	mg := &MigratorPostgres{
		db:         db,
		migrations: migrations,
		flag:       flag,
		schema:     schema,
	}
	if schema == "" {
		mg.schema = "public"
	}
	return mg
}

func (m *MigratorPostgres) Migrate(ctx context.Context) (int64, error) {
	err := m.cleanSchema(ctx)
	if err != nil {
		return 0, stackerr.Wrap(err)
	}
	err = m.createMigrationTable(ctx)
	if err != nil {
		return 0, stackerr.Wrap(err)
	}
	n, err := m.runMigrations(ctx)
	if err != nil {
		return 0, stackerr.Wrap(err)
	}
	return n, nil
}

func (m *MigratorPostgres) cleanSchema(ctx context.Context) error {
	if m.flag&FlagMigrateFresh == 0 {
		return nil
	}
	q := `
		DROP SCHEMA {s} CASCADE;
		CREATE SCHEMA {s};
		GRANT ALL ON SCHEMA {s} TO {s};
		GRANT ALL ON SCHEMA {s} TO {s};
	`
	q = strings.ReplaceAll(q, `{s}`, m.schema)
	_, err := m.db.Exec(ctx, q)
	if err != nil {
		return stackerr.Wrap(err)
	}
	return nil
}

func (m *MigratorPostgres) createMigrationTable(ctx context.Context) error {
	q := `
	CREATE TABLE IF NOT EXISTS %s.migration(
		version BIGINT UNIQUE NOT NULL,
		hash BYTEA NOT NULL
	)
	`
	q = fmt.Sprintf(q, m.schema)
	_, err := m.db.Exec(ctx, q)
	if err != nil {
		return stackerr.Wrap(err)
	}
	return nil
}

func (m *MigratorPostgres) runMigrations(ctx context.Context) (int64, error) {
	lastV, err := m.selectLastVersion(ctx)
	if err != nil {
		return 0, stackerr.Wrap(err)
	}
	var nExec int64
	for _, v := range m.migrations {
		mHash := sha256.Sum256(v.Sql)
		if v.Version <= lastV {
			dbMHash, err := m.selectMigrationHash(ctx, v.Version)
			if err != nil {
				return 0, stackerr.Wrap(err)
			}
			if !bytes.Equal(mHash[:], dbMHash[:]) {
				return 0, fmt.Errorf(
					"migration hash does not match at v%d: %w",
					v.Version, ErrDirtyState,
				)
			}
			continue
		}
		if lastV > -1 &&
			m.flag&(FlagMigrateFresh|FlagMigrateAllowUpgrade) == 0 {
			return 0, nil
		}
		_, err = m.db.Exec(ctx, string(v.Sql))
		if err != nil {
			return 0, fmt.Errorf(
				"executing migration v%d: %w", v.Version, err,
			)
		}
		q := `
		INSERT INTO %s.migration
		(version, hash)
		VALUES
		($1, $2);
		`
		q = fmt.Sprintf(q, m.schema)
		_, err = m.db.Exec(ctx, q, v.Version, mHash[:])
		if err != nil {
			return 0, fmt.Errorf(
				"inserting migration version v%d: %w",
				v.Version,
				err,
			)
		}
		nExec++
	}
	return nExec, nil
}

func (m *MigratorPostgres) selectLastVersion(ctx context.Context) (int64, error) {
	q := `
	SELECT version FROM %s.migration
	ORDER BY version DESC
	LIMIT 1;
	`
	q = fmt.Sprintf(q, m.schema)
	row := m.db.QueryRow(ctx, q)
	var v int64
	err := row.Scan(&v)
	if err != nil && errors.Is(err, sql.ErrNoRows) {
		return -1, nil
	}
	if err != nil {
		return 0, stackerr.Wrap(err)
	}
	return v, nil
}

func (m *MigratorPostgres) selectMigrationHash(
	ctx context.Context, version int64,
) ([32]byte, error) {
	q := `
	SELECT hash FROM %s.migration
	WHERE version = $1;
	`
	q = fmt.Sprintf(q, m.schema)
	row := m.db.QueryRow(ctx, q, version)
	var hash []byte
	err := row.Scan(&hash)
	if err != nil {
		return [32]byte{}, stackerr.Wrap(err)
	}
	return [32]byte(hash), nil
}

type MigratorSqlite struct {
	flag       uint64
	dbFilePath string
	migrations []migration
}

func NewSqlite(
	dbFilePath string, migrations Migrations, flag uint64,
) Migrator {
	return &MigratorSqlite{
		dbFilePath: dbFilePath,
		migrations: migrations,
		flag:       flag,
	}
}

func (m *MigratorSqlite) Migrate(ctx context.Context) (int64, error) {
	err := m.cleanSchema()
	if err != nil {
		return 0, stackerr.Wrap(err)
	}
	db, err := sqlite.New(m.dbFilePath)
	if err != nil {
		return 0, stackerr.Wrap(err)
	}
	defer db.Close(ctx)
	err = m.createMigrationTable(ctx, db)
	if err != nil {
		return 0, stackerr.Wrap(err)
	}
	n, err := m.runMigrations(ctx, db)
	if err != nil {
		return 0, stackerr.Wrap(err)
	}
	return n, nil
}

func (m *MigratorSqlite) cleanSchema() error {
	if m.flag&FlagMigrateFresh == 0 {
		return nil
	}
	err := os.Remove(m.dbFilePath)
	if err != nil && errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return stackerr.Wrap(err)
	}
	return nil
}

func (m *MigratorSqlite) createMigrationTable(
	ctx context.Context, db sql.Database,
) error {
	q := `
	CREATE TABLE IF NOT EXISTS migration(
		version BIGINT UNIQUE NOT NULL,
		hash BYTEA NOT NULL
	);
	`
	_, err := db.Exec(ctx, q)
	if err != nil {
		return stackerr.Wrap(err)
	}
	return nil
}

func (m *MigratorSqlite) runMigrations(
	ctx context.Context, db sql.Database,
) (int64, error) {
	lastV, err := m.selectLastVersion(ctx, db)
	if err != nil {
		return 0, stackerr.Wrap(err)
	}
	var nExec int64
	for _, v := range m.migrations {
		mHash := sha256.Sum256(v.Sql)
		if v.Version <= lastV {
			dbMHash, err := m.selectMigrationHash(
				ctx, db, v.Version,
			)
			if err != nil {
				return 0, stackerr.Wrap(err)
			}
			if !bytes.Equal(mHash[:], dbMHash[:]) {
				return 0, fmt.Errorf(
					"migration hash does not match at v%d: %w",
					v.Version, ErrDirtyState,
				)
			}
			continue
		}
		if lastV > -1 &&
			m.flag&(FlagMigrateFresh|FlagMigrateAllowUpgrade) == 0 {
			return 0, nil
		}
		_, err = db.Exec(ctx, string(v.Sql))
		if err != nil {
			return 0, fmt.Errorf(
				"executing migration v%d: %w", v.Version, err,
			)
		}
		q := `
		INSERT INTO migration
		(version, hash)
		VALUES
		($1, $2);
		`
		_, err = db.Exec(ctx, q, v.Version, mHash[:])
		if err != nil {
			return 0, fmt.Errorf(
				"inserting migration version v%d: %w",
				v.Version,
				err,
			)
		}
		nExec++
	}
	return nExec, nil
}

func (m *MigratorSqlite) selectLastVersion(
	ctx context.Context, db sql.Database,
) (int64, error) {
	q := `
	SELECT version FROM migration
	ORDER BY version DESC
	LIMIT 1;
	`
	row := db.QueryRow(ctx, q)
	var v int64
	err := row.Scan(&v)
	if err != nil && errors.Is(err, sql.ErrNoRows) {
		return -1, nil
	}
	if err != nil {
		return 0, stackerr.Wrap(err)
	}
	return v, nil
}

func (m *MigratorSqlite) selectMigrationHash(
	ctx context.Context, db sql.Database, version int64,
) ([32]byte, error) {
	q := `
	SELECT hash FROM migration
	WHERE version = $1;
	`
	row := db.QueryRow(ctx, q, version)
	var hash []byte
	err := row.Scan(&hash)
	if err != nil {
		return [32]byte{}, stackerr.Wrap(err)
	}
	return [32]byte(hash), nil
}

type migration struct {
	Version int64
	Sql     []byte
}

type Migrations []migration

func (m *Migrations) LoadFromFS(migrationDir string) error {
	errM := "migration: load from fs: %w"
	// get all file names under migrationDir
	ls, err := os.ReadDir(migrationDir)
	if err != nil {
		return fmt.Errorf(errM, err)
	}
	var mf []mFile
	for _, v := range ls {
		// ignore all directories
		if v.IsDir() {
			continue
		}
		// ignore files with underline separator index < 1
		x := strings.Index(v.Name(), "_")
		if x < 1 {
			continue
		}
		// ignore files with version parsing failure
		version, err := strconv.ParseInt(v.Name()[:x], 10, 64)
		if err != nil {
			return fmt.Errorf(errM, err)
		}
		// store remaining filenames and versions
		mf = append(mf, mFile{migrationDir + "/" + v.Name(), version})
	}
	ls = nil
	// sort ascending by version
	sortAsc(mf)
	// transform struct into Migration struct
	r := make([]migration, 0, len(mf))
	for _, v := range mf {
		sql, err := os.ReadFile(v.name)
		if err != nil {
			return fmt.Errorf(errM, err)
		}
		r = append(r, migration{Sql: sql, Version: v.version})
	}
	*m = r
	return nil
}

func (m *Migrations) LoadFromEmbedFS(fs embed.FS, migrationDir string) error {
	errM := "migration: load from embed fs: %w"
	// get all file names under migrationDir
	ls, err := fs.ReadDir(migrationDir)
	if err != nil {
		return fmt.Errorf(errM, err)
	}
	var mf []mFile
	for _, v := range ls {
		// ignore all directories
		if v.IsDir() {
			continue
		}
		// ignore files with underline separator index < 1
		x := strings.Index(v.Name(), "_")
		if x < 1 {
			continue
		}
		// ignore files with version parsing failure
		version, err := strconv.ParseInt(v.Name()[:x], 10, 64)
		if err != nil {
			return fmt.Errorf(errM, err)
		}
		// store remaining filenames and versions
		mf = append(mf, mFile{migrationDir + "/" + v.Name(), version})
	}
	ls = nil
	// sort ascending by version
	sortAsc(mf)
	// transform struct into Migration struct
	r := make([]migration, 0, len(mf))
	for _, v := range mf {
		sql, err := fs.ReadFile(v.name)
		if err != nil {
			return fmt.Errorf(errM, err)
		}
		r = append(r, migration{Sql: sql, Version: v.version})
	}
	*m = r
	return nil
}

type mFile struct {
	name    string
	version int64
}

func sortAsc(m []mFile) {
	l := len(m)
	for i := 0; i < l-1; i++ {
		swapExec := false
		for j := 0; j < l-i-1; j++ {
			if m[j].version < m[j+1].version {
				continue
			}
			swapExec = true
			swap := m[j]
			m[j] = m[j+1]
			m[j+1] = swap
		}
		if !swapExec {
			break
		}
	}
}
