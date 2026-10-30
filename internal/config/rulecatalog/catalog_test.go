// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 alibaba/open-code-review Contributors

package rulecatalog

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func TestLoadDefaultCounts(t *testing.T) {
	cat, err := LoadDefault()
	if err != nil {
		t.Fatalf("LoadDefault() error: %v", err)
	}
	if cat.Schema != Schema {
		t.Errorf("schema = %q, want %q", cat.Schema, Schema)
	}
	if cat.Counts.Rules != 212 {
		t.Errorf("rules = %d, want 212", cat.Counts.Rules)
	}
	if cat.Counts.CPF != 207 {
		t.Errorf("cpf rules = %d, want 207", cat.Counts.CPF)
	}
	if cat.Counts.OCR != 5 {
		t.Errorf("ocr rules = %d, want 5", cat.Counts.OCR)
	}
	if cat.Counts.References != 84 {
		t.Errorf("references = %d, want 84 unique references", cat.Counts.References)
	}
	if cat.Counts.ReferenceCatalogs != 10 {
		t.Errorf("reference catalogs = %d, want 10", cat.Counts.ReferenceCatalogs)
	}
	if got := cat.Len(); got != 212 {
		t.Errorf("Len() = %d, want 212", got)
	}
}

func TestLoadDefaultHasRule(t *testing.T) {
	cat, err := LoadDefault()
	if err != nil {
		t.Fatalf("LoadDefault() error: %v", err)
	}
	for _, id := range []string{"P1-001", "KC5-002", "CXX6-001", "C3-004", "J4-004", "K1", "K5"} {
		if !cat.HasRule(id) {
			t.Errorf("HasRule(%q) = false, want true", id)
		}
	}
	for _, id := range []string{"", "P9-999", "kc5-002", "R-lang-1", "ohos-platform.kmp-platform-overview"} {
		if cat.HasRule(id) {
			t.Errorf("HasRule(%q) = true, want false", id)
		}
	}
}

func TestHasRuleCommaSeparated(t *testing.T) {
	cat, err := LoadDefault()
	if err != nil {
		t.Fatalf("LoadDefault() error: %v", err)
	}
	if !cat.HasRule("P2-001, KC5-002") {
		t.Error("HasRule with two known IDs = false, want true")
	}
	if cat.HasRule("P2-001, P9-999") {
		t.Error("HasRule with one unknown ID = true, want false")
	}
	if cat.HasRule("P2-001,") {
		t.Error("HasRule with an empty entry = true, want false")
	}
}

func TestRuleIDsSorted(t *testing.T) {
	cat, err := LoadDefault()
	if err != nil {
		t.Fatalf("LoadDefault() error: %v", err)
	}
	ids := cat.RuleIDs()
	if len(ids) != 212 {
		t.Fatalf("RuleIDs() length = %d, want 212", len(ids))
	}
	if !sort.StringsAreSorted(ids) {
		t.Error("RuleIDs() is not sorted")
	}
	if ids[0] != "A1-001" {
		t.Errorf("first ID = %q, want A1-001", ids[0])
	}
}

func TestNilCatalog(t *testing.T) {
	var cat *Catalog
	if cat.HasRule("K1") {
		t.Error("nil catalog HasRule = true, want false")
	}
	if cat.RuleIDs() != nil {
		t.Error("nil catalog RuleIDs != nil")
	}
	if cat.Len() != 0 {
		t.Error("nil catalog Len != 0")
	}
	if err := cat.Merge(nil); err != nil {
		t.Errorf("nil catalog Merge(nil) error: %v", err)
	}
}

func TestLoadProjectOverride(t *testing.T) {
	repo := t.TempDir()
	writeProjectCatalog(t, repo, `{
  "schema": "ocr-rule-catalog/v1",
  "version": "test",
  "rules": [
    {
      "id": "K1",
      "namespace": "ocr",
      "prefix": "K",
      "title": "project override",
      "domain": "build-release",
      "severity": "high",
      "check": "project override check",
      "source": {"repo": "project", "path": ".opencodereview/rule.json", "line": 3},
      "status": "active"
    },
    {
      "id": "PROJ-001",
      "namespace": "project",
      "prefix": "PROJ",
      "title": "project rule",
      "domain": "repo",
      "severity": "medium",
      "check": "project check",
      "source": {"repo": "project", "path": ".opencodereview/rule.json", "line": 9},
      "status": "active"
    }
  ]
}`)

	cat, err := Load(repo)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if !cat.HasRule("PROJ-001") {
		t.Error("project rule not loaded")
	}
	if !cat.HasRule("P1-001") {
		t.Error("embedded rule lost after merge")
	}
	if cat.Len() != 213 {
		t.Errorf("rules after merge = %d, want 213", cat.Len())
	}
	if cat.Counts.CPF != 207 || cat.Counts.OCR != 5 {
		t.Errorf("counts after merge = %+v", cat.Counts)
	}
	if cat.ContentHash != "" {
		t.Error("merged catalog kept the generated content hash")
	}
	found := false
	for _, rule := range cat.Rules {
		if rule.ID == "K1" {
			found = true
			if rule.Severity != "high" || rule.Check != "project override check" {
				t.Errorf("K1 override not applied: %+v", rule)
			}
		}
	}
	if !found {
		t.Error("K1 missing after merge")
	}
}

func TestLoadMissingProjectCatalog(t *testing.T) {
	cat, err := Load(t.TempDir())
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cat.Len() != 212 {
		t.Errorf("rules = %d, want 212", cat.Len())
	}
}

func TestLoadEmptyRepoDir(t *testing.T) {
	cat, err := Load("")
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cat.Len() != 212 {
		t.Errorf("rules = %d, want 212", cat.Len())
	}
}

func TestLoadProjectCatalogErrors(t *testing.T) {
	for _, tc := range []struct {
		name    string
		content string
	}{
		{
			name:    "invalid json",
			content: `{`,
		},
		{
			name:    "wrong schema",
			content: `{"schema":"other","rules":[{"id":"X","source":{"repo":"r","path":"p","line":1}}]}`,
		},
		{
			name:    "empty rules",
			content: `{"schema":"ocr-rule-catalog/v1","rules":[]}`,
		},
		{
			name:    "missing source",
			content: `{"schema":"ocr-rule-catalog/v1","rules":[{"id":"X"}]}`,
		},
		{
			name:    "duplicate id",
			content: `{"schema":"ocr-rule-catalog/v1","rules":[{"id":"X","source":{"repo":"r","path":"p","line":1}},{"id":"X","source":{"repo":"r","path":"p","line":2}}]}`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := t.TempDir()
			writeProjectCatalog(t, repo, tc.content)
			if _, err := Load(repo); err == nil {
				t.Fatal("Load() error = nil, want error")
			}
		})
	}
}

func TestLoadProjectCatalogUnreadable(t *testing.T) {
	repo := t.TempDir()
	dir := filepath.Join(repo, ".opencodereview", "rules")
	if err := os.MkdirAll(filepath.Join(dir, "catalog.json"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(repo); err == nil {
		t.Fatal("Load() error = nil, want error for a directory")
	}
}

func TestParseStrictRejectsIncompleteRule(t *testing.T) {
	for _, tc := range []struct {
		name string
		json string
	}{
		{
			name: "missing prefix",
			json: `{"schema":"ocr-rule-catalog/v1","version":"v","rules":[{"id":"A1-001","namespace":"cpf","domain":"d","severity":"high","check":"c","source":{"repo":"r","path":"p","line":1}}]}`,
		},
		{
			name: "missing domain",
			json: `{"schema":"ocr-rule-catalog/v1","version":"v","rules":[{"id":"A1-001","namespace":"cpf","prefix":"A","severity":"high","check":"c","source":{"repo":"r","path":"p","line":1}}]}`,
		},
		{
			name: "missing severity",
			json: `{"schema":"ocr-rule-catalog/v1","version":"v","rules":[{"id":"A1-001","namespace":"cpf","prefix":"A","domain":"d","check":"c","source":{"repo":"r","path":"p","line":1}}]}`,
		},
		{
			name: "missing check",
			json: `{"schema":"ocr-rule-catalog/v1","version":"v","rules":[{"id":"A1-001","namespace":"cpf","prefix":"A","domain":"d","severity":"high","source":{"repo":"r","path":"p","line":1}}]}`,
		},
		{
			name: "missing namespace",
			json: `{"schema":"ocr-rule-catalog/v1","version":"v","rules":[{"id":"A1-001","prefix":"A","domain":"d","severity":"high","check":"c","source":{"repo":"r","path":"p","line":1}}]}`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := parse([]byte(tc.json), true); err == nil {
				t.Fatal("parse(strict) error = nil, want error")
			}
		})
	}
}

func TestValidateReferenceErrors(t *testing.T) {
	for _, tc := range []struct {
		name string
		json string
	}{
		{
			name: "missing reference path",
			json: `{"schema":"ocr-rule-catalog/v1","version":"v","rules":[{"id":"A1-001","namespace":"cpf","prefix":"A","domain":"d","severity":"high","check":"c","source":{"repo":"r","path":"p","line":1}}],"references":[{"id":"r1"}]}`,
		},
		{
			name: "duplicate reference",
			json: `{"schema":"ocr-rule-catalog/v1","version":"v","rules":[{"id":"A1-001","namespace":"cpf","prefix":"A","domain":"d","severity":"high","check":"c","source":{"repo":"r","path":"p","line":1}}],"references":[{"id":"r1","path":"p"},{"id":"r1","path":"q"}]}`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := parse([]byte(tc.json), false); err == nil {
				t.Fatal("parse() error = nil, want error")
			}
		})
	}
}

func TestLoadDefaultVersionNonEmpty(t *testing.T) {
	cat, err := LoadDefault()
	if err != nil {
		t.Fatalf("LoadDefault() error: %v", err)
	}
	if strings.TrimSpace(cat.Version) == "" {
		t.Error("catalog version is empty")
	}
	if len(cat.Namespaces) != 2 {
		t.Errorf("namespaces = %d, want 2", len(cat.Namespaces))
	}
}

func writeProjectCatalog(t *testing.T, repo, content string) {
	t.Helper()
	dir := filepath.Join(repo, ".opencodereview", "rules")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "catalog.json"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
