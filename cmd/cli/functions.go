package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	coreConfig "josex/web/modules/core/config"
	coreServices "josex/web/modules/core/services"
	"josex/web/modules/core/utils"

	"github.com/jackc/pgx/v5"
)

// CmdFunctions handles the `functions` subcommand. `functions dump` writes every live function of
// the ENV_FILE database (default .env.test, freshly reset) into its owner module's
// sql/functions/<name>.sql, the files the migration runner applies on a hash change (INFRA-015).
func CmdFunctions(args []string) {
	fs := flag.NewFlagSet("functions", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Println("🧩 Function files")
		fmt.Println("\nUsage:")
		fmt.Println("  go run ./cmd/cli functions dump")
		fmt.Println("\nRewrites modules/<m>/sql/functions/ from the live functions of DATABASE_URL")
		fmt.Println("(ENV_FILE, default .env.test — run `make db-reset` first). The owner is the module")
		fmt.Println("whose migration defined the function last, in ENABLED_MODULES order.")
	}
	_ = fs.Parse(args)
	if fs.Arg(0) != "dump" {
		fs.Usage()
		os.Exit(1)
	}

	if os.Getenv("ENV_FILE") == "" {
		_ = os.Setenv("ENV_FILE", ".env.test")
	}
	utils.LoadEnv()
	conf := coreConfig.LoadCoreConfig()

	owners, err := functionOwners(conf.EnabledModules)
	if err != nil {
		fmt.Printf("❌ %v\n", err)
		os.Exit(1)
	}

	ctx := context.Background()
	conn, err := pgx.Connect(ctx, conf.DatabaseURL)
	if err != nil {
		fmt.Printf("❌ Connect: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = conn.Close(ctx) }()

	files, unowned, err := functionFiles(ctx, conn, owners)
	if err != nil {
		fmt.Printf("❌ %v\n", err)
		os.Exit(1)
	}

	modules := map[string]int{}
	for path := range files {
		modules[filepath.Dir(path)]++
	}
	for dir := range modules {
		old, _ := filepath.Glob(filepath.Join(dir, "*.sql"))
		for _, f := range old {
			_ = os.Remove(f)
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			fmt.Printf("❌ %v\n", err)
			os.Exit(1)
		}
	}
	for path, body := range files {
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			fmt.Printf("❌ %v\n", err)
			os.Exit(1)
		}
	}

	dirs := make([]string, 0, len(modules))
	for dir := range modules {
		dirs = append(dirs, dir)
	}
	sort.Strings(dirs)
	for _, dir := range dirs {
		fmt.Printf("  %-40s %d files\n", filepath.ToSlash(dir), modules[dir])
	}
	for _, name := range unowned {
		fmt.Printf("⚠️  %s has no migration that creates it — not written\n", name)
	}
	fmt.Printf("✅ %d function files written\n", len(files))
}

var createFunctionPattern = regexp.MustCompile(`(?i)\bCREATE\s+(?:OR\s+REPLACE\s+)?(?:FUNCTION|PROCEDURE)\s+"?(\w+)"?\."?(\w+)"?`)

// functionOwners maps schema.name to the module whose migration created it last, reading the
// migrations in the order they are applied: core first, then ENABLED_MODULES, each by version.
func functionOwners(enabled []string) (map[string]string, error) {
	order := []string{"core"}
	for _, m := range enabled {
		if m != "core" {
			order = append(order, m)
		}
	}
	owners := map[string]string{}
	for _, m := range order {
		files, err := filepath.Glob(filepath.Join("modules", m, "migrations", "*.up.sql"))
		if err != nil {
			return nil, err
		}
		sort.Strings(files)
		for _, f := range files {
			raw, err := os.ReadFile(f)
			if err != nil {
				return nil, err
			}
			for _, match := range createFunctionPattern.FindAllStringSubmatch(string(raw), -1) {
				owners[strings.ToLower(match[1]+"."+match[2])] = m
			}
		}
	}
	return owners, nil
}

// functionFiles renders one file per function name: every overload, its comment and the triggers
// that call it. Keys are file paths.
func functionFiles(ctx context.Context, conn *pgx.Conn, owners map[string]string) (map[string]string, []string, error) {
	rows, err := conn.Query(ctx, `
		SELECT n.nspname, p.proname, p.oid::regprocedure::text,
		       CASE p.prokind WHEN 'p' THEN 'PROCEDURE' ELSE 'FUNCTION' END,
		       pg_get_functiondef(p.oid),
		       obj_description(p.oid, 'pg_proc'),
		       COALESCE((SELECT string_agg(format('DROP TRIGGER IF EXISTS %I ON %s;', t.tgname, t.tgrelid::regclass)
		                                   || E'\n' || pg_get_triggerdef(t.oid) || ';', E'\n\n' ORDER BY t.tgname)
		                 FROM pg_trigger t WHERE t.tgfoid = p.oid AND NOT t.tgisinternal), '')
		FROM pg_proc p
		JOIN pg_namespace n ON n.oid = p.pronamespace
		WHERE n.nspname NOT IN ('pg_catalog', 'information_schema') AND n.nspname NOT LIKE 'pg\_%'
		  AND p.prokind IN ('f', 'p')
		  AND NOT EXISTS (SELECT 1 FROM pg_depend d WHERE d.objid = p.oid AND d.deptype = 'e')
		ORDER BY n.nspname, p.proname, 3`)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	files := map[string]string{}
	schemaOf := map[string]string{} // file path → schema, to catch two schemas sharing a name in a module
	var unowned []string
	for rows.Next() {
		var schema, name, signature, kind, def, triggers string
		var comment *string
		if err := rows.Scan(&schema, &name, &signature, &kind, &def, &comment, &triggers); err != nil {
			return nil, nil, err
		}
		module, ok := owners[schema+"."+name]
		if !ok {
			unowned = append(unowned, signature)
			continue
		}
		path := filepath.Join(coreServices.FunctionsDir(module), name+".sql")
		if prev, ok := schemaOf[path]; ok && prev != schema {
			return nil, nil, fmt.Errorf("%s: both %s.%s and %s.%s would be written there", path, prev, name, schema, name)
		}
		schemaOf[path] = schema

		var b strings.Builder
		if files[path] != "" {
			b.WriteString(files[path])
			b.WriteString("\n")
		}
		b.WriteString(strings.TrimRight(def, "\n"))
		b.WriteString(";\n")
		if comment != nil {
			fmt.Fprintf(&b, "\nCOMMENT ON %s %s IS %s;\n", kind, signature, quoteLiteral(*comment))
		}
		if triggers != "" {
			// Dropped and created again: a constraint trigger has no CREATE OR REPLACE, and the
			// runner applies the whole file on every change.
			fmt.Fprintf(&b, "\n%s\n", triggers)
		}
		// A body from a CRLF migration keeps its \r; the runner applies files with LF only.
		files[path] = strings.ReplaceAll(b.String(), "\r\n", "\n")
	}
	return files, unowned, rows.Err()
}

func quoteLiteral(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}
