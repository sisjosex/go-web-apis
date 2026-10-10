package services

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
)

// A module's functions live one per file in modules/<m>/sql/functions/ (INFRA-015). After the module's
// versioned migrations, every file whose sha256 differs from the one recorded in
// schema_functions_<m> is applied, all in one transaction. Migrations keep the tables; they never
// create a function.

// functionNamePattern finds the routines a file defines, schema-qualified.
var functionNamePattern = regexp.MustCompile(`(?i)\bCREATE\s+(?:OR\s+REPLACE\s+)?(?:FUNCTION|PROCEDURE)\s+([\w"]+)\.([\w"]+)\s*\(`)

// FunctionsDir is where a module keeps its function files.
func FunctionsDir(module string) string {
	return filepath.Join("modules", module, "sql", "functions")
}

type functionFile struct {
	name string // file name without .sql
	sql  string // LF line endings: a CRLF checkout hashes the same as the LF image
	hash string
}

func readFunctionFiles(dir string) ([]functionFile, error) {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var files []functionFile
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		body := strings.ReplaceAll(string(raw), "\r\n", "\n")
		sum := sha256.Sum256([]byte(body))
		files = append(files, functionFile{
			name: strings.TrimSuffix(e.Name(), ".sql"),
			sql:  body,
			hash: hex.EncodeToString(sum[:]),
		})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].name < files[j].name })
	return files, nil
}

// runModuleFunctions applies the module's changed function files and logs one line.
func (ms *MigrationService) runModuleFunctions(module Module) error {
	files, err := readFunctionFiles(FunctionsDir(module.Name))
	if err != nil {
		return fmt.Errorf("failed to read function files: %w", err)
	}
	if len(files) == 0 {
		return nil
	}

	ctx := context.Background() //nolint:forbidigo // startup: migrations run before any request
	tx, err := ms.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	table := "schema_functions_" + module.Name
	// Two processes migrating the same database apply each file once: the second waits here,
	// then finds the hashes already recorded.
	// check:raw-sql the migration runner itself, like golang-migrate: no SP exists before it runs
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, table); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
		name       TEXT PRIMARY KEY,
		sha256     TEXT NOT NULL,
		applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
	)`, table)); err != nil {
		return err
	}

	applied, err := appliedHashes(ctx, tx, table)
	if err != nil {
		return err
	}

	changed := 0
	for _, f := range files {
		if applied[f.name] == f.hash {
			continue
		}
		if err := applyFunctionFile(ctx, tx, f); err != nil {
			return fmt.Errorf("function file %s/%s.sql: %w", FunctionsDir(module.Name), f.name, err)
		}
		// check:raw-sql the migration runner itself, like golang-migrate: no SP exists before it runs
		if _, err := tx.ExecContext(ctx, fmt.Sprintf(`INSERT INTO %s (name, sha256) VALUES ($1, $2)
			ON CONFLICT (name) DO UPDATE SET sha256 = EXCLUDED.sha256, applied_at = now()`, table),
			f.name, f.hash); err != nil {
			return err
		}
		changed++
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	log.Printf("🧩 %s functions: %d applied, %d unchanged", module.Name, changed, len(files)-changed)
	return nil
}

// appliedHashes reads the sha256 recorded for each function file of the module.
func appliedHashes(ctx context.Context, tx *sql.Tx, table string) (map[string]string, error) {
	// check:raw-sql the migration runner itself, like golang-migrate: no SP exists before it runs
	rows, err := tx.QueryContext(ctx, `SELECT name, sha256 FROM `+table)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	applied := map[string]string{}
	for rows.Next() {
		var name, hash string
		if err := rows.Scan(&name, &hash); err != nil {
			return nil, err
		}
		applied[name] = hash
	}
	return applied, rows.Err()
}

// applyFunctionFile runs one file. A same-signature function is replaced in place, so grants and
// dependent triggers and views survive. An overload of the same name the file no longer defines is
// dropped without CASCADE: whatever still depends on it fails the run loudly.
func applyFunctionFile(ctx context.Context, tx *sql.Tx, f functionFile) error {
	matches := functionNamePattern.FindAllStringSubmatch(f.sql, -1)
	schemas := make([]string, 0, len(matches))
	names := make([]string, 0, len(matches))
	for _, m := range matches {
		schemas = append(schemas, strings.ToLower(strings.Trim(m[1], `"`)))
		names = append(names, strings.ToLower(strings.Trim(m[2], `"`)))
	}
	if len(names) == 0 {
		return errors.New("defines no schema-qualified function")
	}

	before, err := routineVersions(ctx, tx, schemas, names)
	if err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `SAVEPOINT function_file`); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, f.sql); err != nil {
		// A changed return type, a renamed parameter or a removed default cannot be replaced in
		// place (42P13): drop the name's overloads and create it again.
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != "42P13" {
			return err
		}
		if _, err := tx.ExecContext(ctx, `ROLLBACK TO SAVEPOINT function_file`); err != nil {
			return err
		}
		if err := dropRoutines(ctx, tx, before, nil); err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, f.sql)
		return err
	}

	// CREATE OR REPLACE rewrites the pg_proc row, so an overload whose xmin did not move is one the
	// file no longer defines.
	after, err := routineVersions(ctx, tx, schemas, names)
	if err != nil {
		return err
	}
	return dropRoutines(ctx, tx, after, before)
}

// dropRoutines drops every overload in sigs; with unchanged set, only those whose xmin it still holds.
func dropRoutines(ctx context.Context, tx *sql.Tx, sigs, unchanged map[string]string) error {
	for sig, xmin := range sigs {
		if prev, ok := unchanged[sig]; unchanged != nil && (!ok || prev != xmin) {
			continue
		}
		if _, err := tx.ExecContext(ctx, `DROP ROUTINE `+sig); err != nil {
			return err
		}
	}
	return nil
}

// routineVersions maps every overload of the named routines (regprocedure text, usable in DROP
// ROUTINE) to its pg_proc row's xmin.
func routineVersions(ctx context.Context, tx *sql.Tx, schemas, names []string) (map[string]string, error) {
	// check:raw-sql the migration runner itself, like golang-migrate: no SP exists before it runs
	rows, err := tx.QueryContext(ctx, `
		SELECT p.oid::regprocedure::text, p.xmin::text
		FROM pg_proc p
		JOIN pg_namespace n ON n.oid = p.pronamespace
		JOIN unnest($1::text[], $2::text[]) AS f(schema_name, proname)
		  ON f.schema_name = n.nspname AND f.proname = p.proname`, schemas, names)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := map[string]string{}
	for rows.Next() {
		var sig, xmin string
		if err := rows.Scan(&sig, &xmin); err != nil {
			return nil, err
		}
		out[sig] = xmin
	}
	return out, rows.Err()
}
