// Command check enforces the mechanical part of .claude/rules/{go,sql,security}.md on the files
// changed against HEAD (staged, unstaged and untracked), or on the whole tree with -all.
//
//	go run ./tools/check        # changed files
//	go run ./tools/check -all   # whole tree
//
// Checks:
//   - gofmt: a changed .go file is gofmt-clean (line endings ignored, the worktree is CRLF).
//   - tenant scope: an SP whose body touches a table that has a tenant_id column takes p_tenant_id.
//   - input DTOs: a *Dto struct never binds TenantID from the client (json:"-" only).
//   - no SQL built with fmt.Sprintf; tenant identity never read from query, path, form or header.
//   - raw SQL: a Go string holding a statement must call an SP (sp_/fn_). Tests keep their direct reads
//     and fixtures in tests/helpers.go; cmd/ and core/testhelpers are exempt; "// check:raw-sql <why>"
//     on the line above opts one statement out (bulk COPY staging). Only lines added against HEAD count.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

var (
	reCreateTable   = regexp.MustCompile(`(?is)CREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?([a-z_]+\.)?([a-z_]+)\s*\((.*?)\);`)
	reTenantColumn  = regexp.MustCompile(`(?im)^\s*tenant_id\s`)
	reCreateFunc    = regexp.MustCompile(`(?is)CREATE\s+(?:OR\s+REPLACE\s+)?FUNCTION\s+([a-z_]+\.)?([a-z_]+)\s*\((.*?)\)\s*RETURNS\s+(\w+).*?\$\$(.*?)\$\$`)
	reSprintfSQL    = regexp.MustCompile(`(?i)fmt\.Sprintf\(\s*"[^"]*\b(SELECT|INSERT|UPDATE|DELETE|FROM)\b`)
	reTenantFromReq = regexp.MustCompile(`\.(Query|Param|PostForm|GetHeader|DefaultQuery)\(\s*"(tenant_id|X-Tenant-Id|tenant)"`)
	reSQLStatement  = regexp.MustCompile(`^\s*(SELECT\s|WITH\s|INSERT\s+INTO\s|UPDATE\s+[a-z_.]+\s+SET\s|DELETE\s+FROM\s|TRUNCATE\s|MERGE\s+INTO\s)`)
	reSPCall        = regexp.MustCompile(`\b(sp|fn)_\w+\s*\(`)
	reUserScoped    = regexp.MustCompile(`(?i)\buser_id\s*=\s*p_user_id\b`)
	reHunk          = regexp.MustCompile(`^@@ -\d+(?:,\d+)? \+(\d+)(?:,(\d+))? @@`)
)

type finding struct {
	file string
	line int
	msg  string
}

func main() {
	all := flag.Bool("all", false, "check the whole tree instead of the files changed against HEAD")
	flag.Parse()

	files, err := targetFiles(*all)
	if err != nil {
		fmt.Fprintln(os.Stderr, "check:", err)
		os.Exit(2)
	}
	if len(files) == 0 {
		fmt.Println("check — no changed files")
		return
	}

	tenantTables, err := tenantScopedTables()
	if err != nil {
		fmt.Fprintln(os.Stderr, "check:", err)
		os.Exit(2)
	}

	var findings []finding
	for _, f := range files {
		switch {
		case strings.HasSuffix(f, ".go"):
			findings = append(findings, checkGo(f)...)
			findings = append(findings, checkRawSQL(f, *all)...)
		case strings.HasSuffix(f, ".up.sql"):
			findings = append(findings, checkSQL(f, tenantTables)...)
		}
	}

	for _, f := range findings {
		fmt.Printf("%s:%d: %s\n", f.file, f.line, f.msg)
	}
	if len(findings) > 0 {
		fmt.Printf("check — %d finding(s)\n", len(findings))
		os.Exit(1)
	}
	fmt.Printf("check — %d file(s) clean\n", len(files))
}

// targetFiles lists .go and .up.sql files: changed against HEAD, or every tracked one with all.
func targetFiles(all bool) ([]string, error) {
	var out []string
	if all {
		lines, err := git("ls-files", "--", "*.go", "*.up.sql")
		if err != nil {
			return nil, err
		}
		out = lines
	} else {
		changed, err := git("diff", "--name-only", "--diff-filter=AM", "HEAD", "--", "*.go", "*.up.sql")
		if err != nil {
			return nil, err
		}
		untracked, err := git("ls-files", "--others", "--exclude-standard", "--", "*.go", "*.up.sql")
		if err != nil {
			return nil, err
		}
		out = append(changed, untracked...)
	}
	var files []string
	for _, f := range out {
		f = filepath.ToSlash(f)
		// tools/ is this program; docs/ is swagger output.
		if f != "" && !strings.HasPrefix(f, "tools/") && !strings.HasPrefix(f, "docs/") {
			files = append(files, f)
		}
	}
	return files, nil
}

func git(args ...string) ([]string, error) {
	cmd := exec.Command("git", append([]string{"-c", "core.safecrlf=false"}, args...)...)
	raw, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return strings.Split(strings.ReplaceAll(strings.TrimSpace(string(raw)), "\r", ""), "\n"), nil
}

// tenantScopedTables collects, from every up migration, the tables declaring a tenant_id column.
func tenantScopedTables() (map[string]bool, error) {
	tables := map[string]bool{}
	err := filepath.WalkDir("modules", func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".up.sql") {
			return err
		}
		src, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		for _, m := range reCreateTable.FindAllStringSubmatch(string(src), -1) {
			if reTenantColumn.MatchString(m[3]) {
				tables[m[2]] = true
			}
		}
		return nil
	})
	return tables, err
}

func checkSQL(file string, tenantTables map[string]bool) []finding {
	src, err := os.ReadFile(file)
	if err != nil {
		return []finding{{file, 0, err.Error()}}
	}
	text := string(src)
	var out []finding
	for _, loc := range reCreateFunc.FindAllStringSubmatchIndex(text, -1) {
		name := text[loc[4]:loc[5]]
		params := strings.ToLower(text[loc[6]:loc[7]])
		returns := strings.ToUpper(text[loc[8]:loc[9]])
		body := text[loc[10]:loc[11]]
		// Trigger functions take no parameters; the row they see is already tenant-scoped.
		if strings.Contains(params, "p_tenant_id") || returns == "TRIGGER" {
			continue
		}
		// A user-scoped membership read — the caller's own tenants — has no tenant to take: its scope is
		// p_user_id, and the body must filter by it (TRACK-032, tenancy.sp_get_user_tenants).
		if strings.Contains(params, "p_user_id") && reUserScoped.MatchString(body) {
			continue
		}
		for table := range tenantTables {
			if regexp.MustCompile(`(?i)\b(FROM|JOIN|UPDATE|INTO|DELETE\s+FROM)\s+(?:[a-z_]+\.)?` + table + `\b`).MatchString(body) {
				line := 1 + strings.Count(text[:loc[0]], "\n")
				out = append(out, finding{file, line, fmt.Sprintf("%s touches tenant-scoped table %q without a p_tenant_id parameter (security.md)", name, table)})
				break
			}
		}
	}
	return out
}

func checkGo(file string) []finding {
	src, err := os.ReadFile(file)
	if err != nil {
		return []finding{{file, 0, err.Error()}}
	}
	normalized := bytes.ReplaceAll(src, []byte("\r\n"), []byte("\n"))

	var out []finding
	if formatted, fmtErr := format.Source(normalized); fmtErr == nil && !bytes.Equal(formatted, normalized) {
		out = append(out, finding{file, 1, "not gofmt-clean — run gofmt on this file (go.md)"})
	}
	isTest := strings.HasSuffix(file, "_test.go") || strings.Contains(file, "/tests/")
	if !isTest {
		out = append(out, checkGoLines(file, normalized)...)
	}
	return append(out, checkDtoTenantBinding(file, normalized)...)
}

func checkGoLines(file string, src []byte) []finding {
	var out []finding
	for i, line := range strings.Split(string(src), "\n") {
		if reSprintfSQL.MatchString(line) {
			out = append(out, finding{file, i + 1, "SQL built with fmt.Sprintf — use $n parameters and named SP arguments (security.md)"})
		}
		if reTenantFromReq.MatchString(line) {
			out = append(out, finding{file, i + 1, "tenant identity read from the request — only c.Get(\"tenant_id\") (security.md)"})
		}
	}
	return out
}

// checkDtoTenantBinding reports a *Dto struct whose TenantID field the client could set.
func checkDtoTenantBinding(file string, src []byte) []finding {
	fset := token.NewFileSet()
	astFile, err := parser.ParseFile(fset, file, src, parser.ParseComments)
	if err != nil {
		return nil
	}
	var out []finding
	ast.Inspect(astFile, func(n ast.Node) bool {
		spec, ok := n.(*ast.TypeSpec)
		if !ok || !strings.HasSuffix(spec.Name.Name, "Dto") {
			return true
		}
		st, ok := spec.Type.(*ast.StructType)
		if !ok {
			return true
		}
		for _, field := range st.Fields.List {
			if !isNamed(field, "TenantID") || (field.Tag != nil && strings.Contains(field.Tag.Value, `json:"-"`)) {
				continue
			}
			out = append(out, finding{file, fset.Position(field.Pos()).Line, spec.Name.Name + ".TenantID must be `json:\"-\"` — set from middleware, never bound from the client (go.md)"})
		}
		return true
	})
	return out
}

func isNamed(field *ast.Field, name string) bool {
	for _, ident := range field.Names {
		if ident.Name == name {
			return true
		}
	}
	return false
}

// checkRawSQL reports a Go string holding a SQL statement that calls no SP, on the lines added against
// HEAD (every line with -all, or when the file is untracked).
func checkRawSQL(file string, all bool) []finding {
	if strings.HasPrefix(file, "cmd/") || strings.HasSuffix(file, "/tests/helpers.go") || strings.Contains(file, "/core/testhelpers/") {
		return nil
	}
	raw, err := os.ReadFile(file)
	if err != nil {
		return []finding{{file, 0, err.Error()}}
	}
	src := bytes.ReplaceAll(raw, []byte("\r\n"), []byte("\n"))
	added, err := addedLines(file, all)
	if err != nil {
		return []finding{{file, 0, err.Error()}}
	}
	fset := token.NewFileSet()
	astFile, err := parser.ParseFile(fset, file, src, 0)
	if err != nil {
		return nil
	}
	lines := strings.Split(string(src), "\n")
	var out []finding
	ast.Inspect(astFile, func(n ast.Node) bool {
		lit, ok := n.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return true
		}
		line := fset.Position(lit.Pos()).Line
		if (added == nil || added[line]) && isRawStatement(lit.Value) && !optedOut(lines, line) {
			out = append(out, finding{file, line, rawSQLMessage(file)})
		}
		return true
	})
	return out
}

func isRawStatement(literal string) bool {
	value, err := strconv.Unquote(literal)
	if err != nil {
		return false
	}
	return reSQLStatement.MatchString(value) && !reSPCall.MatchString(value)
}

// optedOut: one of the two lines above the statement says "// check:raw-sql <why>".
func optedOut(lines []string, line int) bool {
	for i := line - 2; i >= 0 && i >= line-3; i-- {
		if strings.Contains(lines[i], "// check:raw-sql ") {
			return true
		}
	}
	return false
}

func rawSQLMessage(file string) string {
	if strings.HasSuffix(file, "_test.go") || strings.Contains(file, "/tests/") {
		return "direct SQL in a test — arrange through the API or an SP; a fixture or read the API cannot give is a named helper in tests/helpers.go (go-tests.md)"
	}
	return "direct SQL — a repository calls one SP; move the statement into a schema.sp_* (go.md)"
}

// addedLines answers the line numbers added against HEAD; nil means every line counts.
func addedLines(file string, all bool) (map[int]bool, error) {
	if all {
		return nil, nil
	}
	if tracked, _ := git("ls-files", "--", file); len(tracked) == 0 || tracked[0] == "" {
		return nil, nil
	}
	diff, err := git("diff", "-U0", "HEAD", "--", file)
	if err != nil {
		return nil, err
	}
	added := map[int]bool{}
	for _, h := range diff {
		m := reHunk.FindStringSubmatch(h)
		if m == nil {
			continue
		}
		start, _ := strconv.Atoi(m[1])
		count := 1
		if m[2] != "" {
			count, _ = strconv.Atoi(m[2])
		}
		for l := start; l < start+count; l++ {
			added[l] = true
		}
	}
	return added, nil
}
