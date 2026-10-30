// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 alibaba/open-code-review Contributors

package tool

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/alibaba/open-code-review/internal/config/rulecatalog"
	"github.com/alibaba/open-code-review/internal/model"
)

func TestParseComments(t *testing.T) {
	tests := []struct {
		name      string
		args      map[string]any
		wantCount int
		wantErr   bool
	}{
		{
			name: "valid comments array with per-item path",
			args: map[string]any{
				"comments": []any{
					map[string]any{"content": "issue 1", "existing_code": "old", "path": "main.go"},
					map[string]any{"content": "issue 2", "suggestion_code": "new", "path": "main.go"},
				},
			},
			wantCount: 2,
		},
		{
			name: "comments as JSON string",
			args: map[string]any{
				"comments": `[{"content":"from string","path":"main.go"}]`,
			},
			wantCount: 1,
		},
		{
			name: "missing path skips comment",
			args: map[string]any{
				"comments": []any{
					map[string]any{"content": "no path"},
				},
			},
			wantCount: 0,
		},
		{
			name: "missing content skips comment",
			args: map[string]any{
				"comments": []any{
					map[string]any{"existing_code": "has no content", "path": "file.go"},
				},
			},
			wantCount: 0,
		},
		{
			name:    "empty comments array returns error",
			args:    map[string]any{"comments": []any{}},
			wantErr: true,
		},
		{
			name:    "no comments key returns error",
			args:    map[string]any{},
			wantErr: true,
		},
		{
			name:    "invalid JSON string returns error",
			args:    map[string]any{"comments": "not json"},
			wantErr: true,
		},
		{
			name: "thinking field preserved",
			args: map[string]any{
				"comments": []any{
					map[string]any{"content": "c", "thinking": "my reasoning", "path": "a.go"},
				},
			},
			wantCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			comments, errMsg := ParseComments(tt.args)
			if tt.wantErr {
				if errMsg == "" {
					t.Error("expected error message, got empty")
				}
				return
			}
			if errMsg != "" {
				t.Fatalf("unexpected error: %s", errMsg)
			}
			if len(comments) != tt.wantCount {
				t.Errorf("len(comments) = %d, want %d", len(comments), tt.wantCount)
			}
		})
	}
}

func TestParseComments_Fields(t *testing.T) {
	args := map[string]any{
		"comments": []any{
			map[string]any{
				"content":         "fix null check",
				"existing_code":   "if (x == null)",
				"suggestion_code": "if (x === null)",
				"thinking":        "strict equality is safer",
				"path":            "src/app.ts",
			},
		},
	}
	comments, errMsg := ParseComments(args)
	if errMsg != "" {
		t.Fatal(errMsg)
	}
	if len(comments) != 1 {
		t.Fatal("expected 1 comment")
	}
	c := comments[0]
	if c.Path != "src/app.ts" {
		t.Errorf("Path = %q", c.Path)
	}
	if c.Content != "fix null check" {
		t.Errorf("Content = %q", c.Content)
	}
	if c.ExistingCode != "if (x == null)" {
		t.Errorf("ExistingCode = %q", c.ExistingCode)
	}
	if c.SuggestionCode != "if (x === null)" {
		t.Errorf("SuggestionCode = %q", c.SuggestionCode)
	}
	if c.Thinking != "strict equality is safer" {
		t.Errorf("Thinking = %q", c.Thinking)
	}
}

// TestParseComments_CategorySeverity verifies the structured category and severity
// fields are read off each comment object when present, and left zero-valued when
// absent (older/less-capable models that omit them still produce valid comments).
func TestParseComments_CategorySeverity(t *testing.T) {
	t.Run("parsed when present", func(t *testing.T) {
		args := map[string]any{
			"comments": []any{
				map[string]any{
					"content":       "Potential nil pointer dereference.",
					"existing_code": "x := *p",
					"category":      "bug",
					"severity":      "high",
					"path":          "main.go",
				},
			},
		}
		comments, errMsg := ParseComments(args)
		if errMsg != "" {
			t.Fatalf("unexpected error message: %s", errMsg)
		}
		if len(comments) != 1 {
			t.Fatalf("expected 1 comment, got %d", len(comments))
		}
		if got := comments[0].Category; got != "bug" {
			t.Errorf("category = %q, want %q", got, "bug")
		}
		if got := comments[0].Severity; got != "high" {
			t.Errorf("severity = %q, want %q", got, "high")
		}
	})

	t.Run("zero-valued when absent", func(t *testing.T) {
		args := map[string]any{
			"comments": []any{
				map[string]any{
					"content":       "Consider renaming for clarity.",
					"existing_code": "a := 1",
					"path":          "main.go",
				},
			},
		}
		comments, errMsg := ParseComments(args)
		if errMsg != "" {
			t.Fatalf("unexpected error message: %s", errMsg)
		}
		if len(comments) != 1 {
			t.Fatalf("expected 1 comment, got %d", len(comments))
		}
		if comments[0].Category != "" {
			t.Errorf("category = %q, want empty", comments[0].Category)
		}
		if comments[0].Severity != "" {
			t.Errorf("severity = %q, want empty", comments[0].Severity)
		}
		if comments[0].Disposition != "" {
			t.Errorf("disposition = %q, want empty", comments[0].Disposition)
		}
	})

	t.Run("normalizes casing", func(t *testing.T) {
		args := map[string]any{
			"comments": []any{
				map[string]any{
					"content":       "Potential nil pointer dereference.",
					"existing_code": "x := *p",
					"category":      "Security",
					"severity":      "Critical",
					"path":          "main.go",
				},
			},
		}
		comments, errMsg := ParseComments(args)
		if errMsg != "" {
			t.Fatalf("unexpected error message: %s", errMsg)
		}
		if len(comments) != 1 {
			t.Fatalf("expected 1 comment, got %d", len(comments))
		}
		if got := comments[0].Category; got != "security" {
			t.Errorf("category = %q, want %q", got, "security")
		}
		if got := comments[0].Severity; got != "critical" {
			t.Errorf("severity = %q, want %q", got, "critical")
		}
	})
}

func TestParseComments_CategorySeveritySchemaDrift(t *testing.T) {
	args := map[string]any{
		"comments": []any{
			map[string]any{
				"content":       "Use the canonical metadata fallback.",
				"existing_code": "value := compute()",
				"category":      "correctness",
				"severity":      "info",
				"path":          "main.go",
			},
		},
	}

	comments, errMsg := ParseComments(args)
	if errMsg != "" {
		t.Fatalf("unexpected error message: %s", errMsg)
	}
	if len(comments) != 1 {
		t.Fatalf("expected 1 comment, got %d", len(comments))
	}
	if got := comments[0].Category; got != "other" {
		t.Errorf("category = %q, want %q", got, "other")
	}
	if got := comments[0].Severity; got != "suggestion" {
		t.Errorf("severity = %q, want %q", got, "suggestion")
	}
	if got := comments[0].Content; got != "Use the canonical metadata fallback." {
		t.Errorf("content = %q", got)
	}
}

func TestParseComments_LegacySeverityMigration(t *testing.T) {
	tests := []struct {
		name            string
		severity        string
		disposition     string
		wantSeverity    string
		wantDisposition string
	}{
		{"low becomes suggestion", "low", "", "suggestion", ""},
		{"needs_confirmation becomes medium plus disposition", "needs_confirmation", "", "medium", "needs_confirmation"},
		{"explicit disposition is parsed", "high", "needs_confirmation", "high", "needs_confirmation"},
		{"explicit confirmed overrides legacy inference", "needs_confirmation", "confirmed", "medium", "confirmed"},
		{"unknown disposition defaults to confirmed", "medium", "unclear", "medium", "confirmed"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj := map[string]any{
				"content":       "finding",
				"existing_code": "x := 1",
				"severity":      tt.severity,
				"path":          "main.go",
			}
			if tt.disposition != "" {
				obj["disposition"] = tt.disposition
			}
			comments, errMsg := ParseComments(map[string]any{"comments": []any{obj}})
			if errMsg != "" {
				t.Fatalf("unexpected error message: %s", errMsg)
			}
			if len(comments) != 1 {
				t.Fatalf("expected 1 comment, got %d", len(comments))
			}
			if got := comments[0].Severity; got != tt.wantSeverity {
				t.Errorf("severity = %q, want %q", got, tt.wantSeverity)
			}
			if got := comments[0].Disposition; got != tt.wantDisposition {
				t.Errorf("disposition = %q, want %q", got, tt.wantDisposition)
			}
		})
	}
}

func TestParseComments_RuleCatalogValidation(t *testing.T) {
	cat, err := rulecatalog.LoadDefault()
	if err != nil {
		t.Fatalf("LoadDefault() error: %v", err)
	}
	for _, tt := range []struct {
		name            string
		ruleID          string
		catalog         *rulecatalog.Catalog
		wantRuleID      string
		wantDisposition string
	}{
		{"known ID is kept", "P1-001", cat, "P1-001", "confirmed"},
		{"unknown ID is dropped and downgraded", "P9-999", cat, "", "needs_confirmation"},
		{"comma-separated known IDs are kept", "P2-001, KC5-002", cat, "P2-001, KC5-002", "confirmed"},
		{"one unknown ID drops the whole list", "P2-001, P9-999", cat, "", "needs_confirmation"},
		{"no catalog keeps the ID", "P9-999", nil, "P9-999", "confirmed"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			obj := map[string]any{
				"content":       "finding",
				"existing_code": "x := 1",
				"severity":      "high",
				"disposition":   "confirmed",
				"rule_id":       tt.ruleID,
				"path":          "main.go",
			}
			comments, _, errMsg := parseCommentsInner(map[string]any{"comments": []any{obj}}, "", tt.catalog)
			if errMsg != "" {
				t.Fatalf("unexpected error message: %s", errMsg)
			}
			if len(comments) != 1 {
				t.Fatalf("expected 1 comment, got %d", len(comments))
			}
			if got := comments[0].RuleID; got != tt.wantRuleID {
				t.Errorf("rule_id = %q, want %q", got, tt.wantRuleID)
			}
			if got := comments[0].Disposition; got != tt.wantDisposition {
				t.Errorf("disposition = %q, want %q", got, tt.wantDisposition)
			}
		})
	}
}

func TestCodeCommentProvider_CatalogValidation(t *testing.T) {
	cat, err := rulecatalog.LoadDefault()
	if err != nil {
		t.Fatalf("LoadDefault() error: %v", err)
	}
	collector := NewCommentCollector()
	p := &CodeCommentProvider{Collector: collector, Catalog: cat}
	args := map[string]any{"comments": []any{map[string]any{
		"content":       "finding",
		"existing_code": "x := 1",
		"severity":      "high",
		"rule_id":       "P9-999",
		"path":          "main.go",
	}}}
	if _, err := p.Execute(context.Background(), args); err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	comments := collector.Comments()
	if len(comments) != 1 {
		t.Fatalf("comments = %d, want 1", len(comments))
	}
	if comments[0].RuleID != "" {
		t.Errorf("rule_id = %q, want empty", comments[0].RuleID)
	}
	if comments[0].Disposition != "needs_confirmation" {
		t.Errorf("disposition = %q, want needs_confirmation", comments[0].Disposition)
	}
}

func TestRegistryRuleCatalog(t *testing.T) {
	cat, err := rulecatalog.LoadDefault()
	if err != nil {
		t.Fatalf("LoadDefault() error: %v", err)
	}
	reg := NewRegistry()
	if got := reg.RuleCatalog(); got != nil {
		t.Error("empty registry RuleCatalog() != nil")
	}
	reg.Register(&CodeCommentProvider{Collector: NewCommentCollector(), Catalog: cat})
	if got := reg.RuleCatalog(); got != cat {
		t.Error("registry did not return the provider catalog")
	}
	reg2 := NewRegistry()
	reg2.Register(&FileReadProvider{})
	if got := reg2.RuleCatalog(); got != nil {
		t.Error("non-comment provider RuleCatalog() != nil")
	}
}

func TestParseCommentsWithPathAndCatalog(t *testing.T) {
	cat, err := rulecatalog.LoadDefault()
	if err != nil {
		t.Fatalf("LoadDefault() error: %v", err)
	}
	args := map[string]any{"comments": []any{map[string]any{
		"content":       "finding",
		"existing_code": "x := 1",
		"severity":      "high",
		"rule_id":       "NOT-IN-CATALOG",
		"path":          "main.go",
	}}}
	comments, repair, errMsg := ParseCommentsWithPathAndCatalog(args, "", cat)
	if errMsg != "" || repair != nil {
		t.Fatalf("unexpected parse result: errMsg=%q repair=%+v", errMsg, repair)
	}
	if len(comments) != 1 {
		t.Fatalf("comments = %d, want 1", len(comments))
	}
	if comments[0].RuleID != "" || comments[0].Disposition != "needs_confirmation" {
		t.Errorf("comment = %+v, want dropped rule ID and needs_confirmation", comments[0])
	}
}

// TestLlmComment_JSONCategorySeverity verifies category and severity serialize as
// flat siblings alongside content when set, and are omitted entirely when empty so
// existing JSON consumers are unaffected.
func TestLlmComment_JSONCategorySeverity(t *testing.T) {
	t.Run("omitted when empty", func(t *testing.T) {
		b, err := json.Marshal(model.LlmComment{Path: "main.go", Content: "no metadata"})
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		out := string(b)
		if strings.Contains(out, "category") {
			t.Errorf("expected no category key, got %s", out)
		}
		if strings.Contains(out, "severity") {
			t.Errorf("expected no severity key, got %s", out)
		}
		if strings.Contains(out, "disposition") {
			t.Errorf("expected no disposition key, got %s", out)
		}
	})

	t.Run("serialized when set", func(t *testing.T) {
		b, err := json.Marshal(model.LlmComment{
			Path:        "main.go",
			Content:     "sql injection",
			Category:    "security",
			Severity:    "critical",
			Disposition: "needs_confirmation",
		})
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		out := string(b)
		if !strings.Contains(out, `"category":"security"`) {
			t.Errorf("expected category in output, got %s", out)
		}
		if !strings.Contains(out, `"severity":"critical"`) {
			t.Errorf("expected severity in output, got %s", out)
		}
		if !strings.Contains(out, `"disposition":"needs_confirmation"`) {
			t.Errorf("expected disposition in output, got %s", out)
		}
	})
}

func TestCodeCommentProvider_Execute(t *testing.T) {
	t.Run("adds comments to collector", func(t *testing.T) {
		collector := NewCommentCollector()
		p := &CodeCommentProvider{Collector: collector}
		result, err := p.Execute(context.Background(), map[string]any{
			"comments": []any{
				map[string]any{"content": "issue 1", "path": "main.go"},
				map[string]any{"content": "issue 2", "path": "main.go"},
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		if result != CommentSucceed {
			t.Errorf("result = %q, want %q", result, CommentSucceed)
		}
		if len(collector.Comments()) != 2 {
			t.Errorf("collector has %d comments, want 2", len(collector.Comments()))
		}
	})

	t.Run("all entries dropped returns error message", func(t *testing.T) {
		collector := NewCommentCollector()
		p := &CodeCommentProvider{Collector: collector}
		result, err := p.Execute(context.Background(), map[string]any{
			"comments": []any{
				map[string]any{"path": "main.go", "message": "nil dereference"},
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		if result != CommentNoneUsableMsg {
			t.Errorf("result = %q, want %q", result, CommentNoneUsableMsg)
		}
		if len(collector.Comments()) != 0 {
			t.Errorf("collector has %d comments, want 0", len(collector.Comments()))
		}
	})

	t.Run("partially dropped batch still succeeds", func(t *testing.T) {
		collector := NewCommentCollector()
		p := &CodeCommentProvider{Collector: collector}
		result, err := p.Execute(context.Background(), map[string]any{
			"comments": []any{
				map[string]any{"content": "real finding", "path": "main.go"},
				map[string]any{"path": "main.go", "message": "dropped: no content"},
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		if result != CommentSucceed {
			t.Errorf("result = %q, want %q", result, CommentSucceed)
		}
		if len(collector.Comments()) != 1 {
			t.Errorf("collector has %d comments, want 1", len(collector.Comments()))
		}
	})

	t.Run("nil collector returns error message", func(t *testing.T) {
		p := &CodeCommentProvider{Collector: nil}
		result, err := p.Execute(context.Background(), map[string]any{
			"comments": []any{map[string]any{"content": "x", "path": "main.go"}},
		})
		if err != nil {
			t.Fatal(err)
		}
		if result == CommentSucceed {
			t.Error("expected error message for nil collector")
		}
	})

	t.Run("invalid args returns error message", func(t *testing.T) {
		collector := NewCommentCollector()
		p := &CodeCommentProvider{Collector: collector}
		result, err := p.Execute(context.Background(), map[string]any{})
		if err != nil {
			t.Fatal(err)
		}
		if result == CommentSucceed {
			t.Error("expected error message for empty args")
		}
	})

	t.Run("tool type is CodeComment", func(t *testing.T) {
		p := &CodeCommentProvider{}
		if p.Tool() != CodeComment {
			t.Errorf("Tool() = %v, want CodeComment", p.Tool())
		}
	})
}

func TestParseComments_NormalizePath(t *testing.T) {
	args := map[string]any{
		"comments": []any{
			map[string]any{"path": `pkg\util.go`, "content": "fix 1"},
			map[string]any{"path": `./pkg/util.go`, "content": "fix 2"},
			map[string]any{"path": `pkg/sub//util.go`, "content": "fix 3"},
		},
	}
	comments, errMsg := ParseComments(args)
	if errMsg != "" || len(comments) != 3 {
		t.Fatalf("ParseComments failed: err=%q len=%d", errMsg, len(comments))
	}
	for i, want := range []string{"pkg/util.go", "pkg/util.go", "pkg/sub/util.go"} {
		if comments[i].Path != want {
			t.Errorf("comment[%d].Path = %q, want %q", i, comments[i].Path, want)
		}
	}
}
