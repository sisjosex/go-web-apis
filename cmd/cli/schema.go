package main

import (
	"bytes"
	"flag"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	coreConfig "josex/web/modules/core/config"
	"josex/web/modules/core/utils"
)

// CmdSchema handles the `schema` subcommand. `schema dump` writes modules/<m>/sql/schema.sql for every
// module that owns a schema of its name: tables, indexes, constraints, views and triggers — never a
// function, those live in sql/functions (INFRA-015). The gate regenerates it after the tests and
// fails on a diff.
func CmdSchema(args []string) {
	fs := flag.NewFlagSet("schema", flag.ExitOnError)
	container := fs.String("container", os.Getenv("PG_DUMP_CONTAINER"),
		"run pg_dump inside this docker container (its version matches the server's)")
	fs.Usage = func() {
		fmt.Println("🗂️  Schema snapshot")
		fmt.Println("\nUsage:")
		fmt.Println("  go run ./cmd/cli schema dump [-container tenant-db]")
		fmt.Println("\nRewrites modules/<m>/sql/schema.sql from DATABASE_URL (ENV_FILE, default .env.test —")
		fmt.Println("`make schema` resets it first). pg_dump must match the server's major version;")
		fmt.Println("-container (or PG_DUMP_CONTAINER) runs the one inside that container.")
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

	written := 0
	for _, module := range conf.EnabledModules {
		if _, err := os.Stat(filepath.Join("modules", module, "migrations")); err != nil {
			continue
		}
		dump, err := pgDumpSchema(conf.DatabaseURL, *container, module)
		if err != nil {
			fmt.Printf("❌ %s: %v\n", module, err)
			os.Exit(1)
		}
		if dump == "" {
			continue // the module keeps its tables in another module's schema
		}
		path := filepath.Join("modules", module, "sql", "schema.sql")
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			fmt.Printf("❌ %v\n", err)
			os.Exit(1)
		}
		if err := os.WriteFile(path, []byte(dump), 0o644); err != nil {
			fmt.Printf("❌ %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("  %s\n", filepath.ToSlash(path))
		written++
	}
	fmt.Printf("✅ %d schema snapshots written\n", written)
}

// pgDumpSchema dumps one schema without functions, owners, privileges or the dump's own version
// lines, so the file changes only when the schema does. An absent schema dumps as "".
func pgDumpSchema(databaseURL, container, schema string) (string, error) {
	flags := []string{"--schema-only", "--no-owner", "--no-privileges", "--schema=" + schema,
		// vehicle_positions has one partition per UTC day, created relative to today.
		"--exclude-table=tracking.vehicle_positions_p*"}

	var cmd *exec.Cmd
	if container != "" {
		u, err := url.Parse(databaseURL)
		if err != nil {
			return "", err
		}
		password, _ := u.User.Password()
		args := append([]string{"exec", "-e", "PGPASSWORD=" + password, container, "pg_dump",
			"-U", u.User.Username(), "-d", strings.TrimPrefix(u.Path, "/")}, flags...)
		cmd = exec.Command("docker", args...)
	} else {
		cmd = exec.Command("pg_dump", append(flags, "--dbname="+databaseURL)...)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("pg_dump: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return cleanSchemaDump(string(out)), nil
}

var (
	tocHeader        = regexp.MustCompile(`^-- Name: (.*); Type: ([A-Z ]+); Schema: `)
	versionLine      = regexp.MustCompile(`^-- Dumped (from database|by pg_dump) version |^\\(un)?restrict `)
	createsSomething = regexp.MustCompile(`(?m)^CREATE (TABLE|VIEW|MATERIALIZED VIEW|SEQUENCE|TYPE|DOMAIN|INDEX|UNIQUE INDEX) `)
)

// cleanSchemaDump drops every TOC entry that is a function, a procedure or a comment on one, and
// the version lines. A dump with nothing created but the schema itself comes back as "".
func cleanSchemaDump(dump string) string {
	lines := strings.Split(strings.ReplaceAll(dump, "\r\n", "\n"), "\n")
	var out []string
	skip := false
	for i, line := range lines {
		// A TOC entry starts "--" / "-- Name: …; Type: …" / "--".
		if line == "--" && i+1 < len(lines) {
			if m := tocHeader.FindStringSubmatch(lines[i+1]); m != nil {
				kind, name := m[2], m[1]
				skip = kind == "FUNCTION" || kind == "PROCEDURE" ||
					(kind == "COMMENT" && (strings.HasPrefix(name, "FUNCTION ") || strings.HasPrefix(name, "PROCEDURE ")))
			}
		}
		if skip || versionLine.MatchString(line) {
			continue
		}
		out = append(out, line)
	}
	text := strings.TrimRight(strings.Join(out, "\n"), "\n") + "\n"
	if !createsSomething.MatchString(text) {
		return ""
	}
	return text
}
