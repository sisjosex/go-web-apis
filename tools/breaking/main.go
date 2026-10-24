// Command breaking fails when docs/openapi.json breaks a client against the committed one (INFRA-021):
// a removed route, field or parameter, a narrowed type, a new required input. oasdiff judges; an
// operation listed in docs/breaking.allow passes, so a deliberate break is a reviewed line, not a
// surprise for the phone app that is already installed.
//
//	go run ./tools/breaking
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

const (
	contractPath = "docs/openapi.json"
	allowPath    = "docs/breaking.allow"
	oasdiff      = "github.com/oasdiff/oasdiff@v1.34.1"
	levelError   = 3
)

type change struct {
	ID        string `json:"id"`
	Text      string `json:"text"`
	Level     int    `json:"level"`
	Operation string `json:"operation"`
	Path      string `json:"path"`
}

func main() {
	basePathFlag := flag.String("base", "", "compare against this file instead of HEAD's contract")
	newPath := flag.String("new", contractPath, "the contract to check")
	flag.Parse()

	var base []byte
	var err error
	if *basePathFlag != "" {
		base, err = os.ReadFile(*basePathFlag)
	} else {
		base, err = exec.Command("git", "show", "HEAD:"+contractPath).Output()
	}
	if err != nil {
		fmt.Println("breaking: no committed contract yet — nothing to compare")
		return
	}
	dir, err := os.MkdirTemp("", "breaking")
	if err != nil {
		fail(err)
	}
	defer func() { _ = os.RemoveAll(dir) }()
	basePath := filepath.Join(dir, "base.json")
	if err := os.WriteFile(basePath, base, 0o644); err != nil {
		fail(err)
	}

	var stdout, stderr bytes.Buffer
	cmd := exec.Command("go", "run", oasdiff, "breaking", basePath, *newPath, "--format", "json")
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		fail(fmt.Errorf("oasdiff: %v\n%s", err, stderr.String()))
	}
	var changes []change
	if out := bytes.TrimSpace(stdout.Bytes()); len(out) > 0 {
		if err := json.Unmarshal(out, &changes); err != nil {
			fail(fmt.Errorf("oasdiff output: %v", err))
		}
	}

	allowed, err := readAllow()
	if err != nil {
		fail(err)
	}
	var refused, passed []string
	for _, c := range changes {
		if c.Level < levelError {
			continue
		}
		line := fmt.Sprintf("%s %s — %s", c.Operation, c.Path, c.Text)
		if allowed[c.Operation+" "+c.Path] {
			passed = append(passed, line)
			continue
		}
		refused = append(refused, line)
	}
	sort.Strings(refused)
	for _, p := range passed {
		fmt.Println("breaking (allowed): " + p)
	}
	for _, r := range refused {
		fmt.Println("breaking: " + r)
	}
	if len(refused) > 0 {
		fmt.Printf("breaking — %d change(s) break a client. Keep the old shape (add, do not rename or remove), or list the operation in %s as `METHOD /path  # KEY why`.\n", len(refused), allowPath)
		os.Exit(1)
	}
	fmt.Printf("breaking — no client broken (%d change(s) checked)\n", len(changes))
}

// readAllow answers the "METHOD /path" lines of docs/breaking.allow; `#` starts a comment.
func readAllow() (map[string]bool, error) {
	out := map[string]bool{}
	f, err := os.Open(allowPath)
	if os.IsNotExist(err) {
		return out, nil
	}
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line, _, _ := strings.Cut(sc.Text(), "#")
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			out[strings.ToUpper(fields[0])+" "+fields[1]] = true
		}
	}
	return out, sc.Err()
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "breaking:", err)
	os.Exit(2)
}
