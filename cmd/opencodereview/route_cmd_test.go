// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 alibaba/open-code-review Contributors

package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alibaba/open-code-review/internal/stdout"
)

func TestRunRulesRoute(t *testing.T) {
	dir := t.TempDir()
	runGitForRouteTest(t, dir, "init")
	writeRouteTestFile(t, filepath.Join(dir, "main.go"), "package main\n")
	writeRouteTestFile(t, filepath.Join(dir, ".opencodereview", "domain-routes.json"), `{
  "schema": "ocr-domain-routes/v1",
  "domains": [
    {"id": "go-domain", "priority": 200, "paths": ["**/*.go"], "sources": ["test"], "checks": ["go check"]}
  ]
}`)

	oldRepo := rulesRouteRepoDir
	rulesRouteRepoDir = dir
	t.Cleanup(func() { rulesRouteRepoDir = oldRepo })

	var buf bytes.Buffer
	defer stdout.Swap(&buf)()
	if err := runRulesRoute([]string{"main.go"}); err != nil {
		t.Fatalf("runRulesRoute: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"Domain routing: enabled=true", "File: main.go", "- go-domain", "checks: 1", "Injected:"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestRunRulesRouteNoMatch(t *testing.T) {
	dir := t.TempDir()
	runGitForRouteTest(t, dir, "init")
	writeRouteTestFile(t, filepath.Join(dir, "README.md"), "plain prose\n")

	oldRepo := rulesRouteRepoDir
	rulesRouteRepoDir = dir
	t.Cleanup(func() { rulesRouteRepoDir = oldRepo })

	var buf bytes.Buffer
	defer stdout.Swap(&buf)()
	if err := runRulesRoute([]string{"README.md"}); err != nil {
		t.Fatalf("runRulesRoute: %v", err)
	}
	if !strings.Contains(buf.String(), "No domain matched.") {
		t.Errorf("output missing no-match notice:\n%s", buf.String())
	}
}

func TestRunRulesRouteDisabled(t *testing.T) {
	dir := t.TempDir()
	runGitForRouteTest(t, dir, "init")
	writeRouteTestFile(t, filepath.Join(dir, ".opencodereview", "domain-routes.json"), `{
  "schema": "ocr-domain-routes/v1",
  "enabled": false
}`)

	oldRepo := rulesRouteRepoDir
	rulesRouteRepoDir = dir
	t.Cleanup(func() { rulesRouteRepoDir = oldRepo })

	var buf bytes.Buffer
	defer stdout.Swap(&buf)()
	if err := runRulesRoute([]string{"main.go"}); err != nil {
		t.Fatalf("runRulesRoute: %v", err)
	}
	if !strings.Contains(buf.String(), "Domain routing is disabled") {
		t.Errorf("output missing disabled notice:\n%s", buf.String())
	}
}

func runGitForRouteTest(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func writeRouteTestFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
