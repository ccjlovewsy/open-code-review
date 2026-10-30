// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 alibaba/open-code-review Contributors

package domainroute

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/alibaba/open-code-review/internal/config/rulecatalog"
)

func TestLoadDefault(t *testing.T) {
	r, err := LoadDefault()
	if err != nil {
		t.Fatalf("LoadDefault() error: %v", err)
	}
	if !r.Enabled() {
		t.Fatal("default router is disabled")
	}
	if r.MaxDomains() != DefaultMaxDomains {
		t.Errorf("MaxDomains() = %d, want %d", r.MaxDomains(), DefaultMaxDomains)
	}
	if r.TokenBudget() != DefaultTokenBudget {
		t.Errorf("TokenBudget() = %d, want %d", r.TokenBudget(), DefaultTokenBudget)
	}
	domains := r.Domains()
	if len(domains) != 10 {
		t.Fatalf("Domains() = %d, want 10", len(domains))
	}
	want := map[string]bool{
		"ohos-platform": false, "crt": false, "gc": false, "interop": false,
		"gradle": false, "architecture": false, "build-release": false,
		"memory": false, "cpp": false, "kotlin": false,
	}
	for _, d := range domains {
		if _, ok := want[d.ID]; !ok {
			t.Errorf("unexpected domain %q", d.ID)
			continue
		}
		want[d.ID] = true
	}
	for id, seen := range want {
		if !seen {
			t.Errorf("missing domain %q", id)
		}
	}
}

func TestDefaultRoutesAreConsistent(t *testing.T) {
	r, err := LoadDefault()
	if err != nil {
		t.Fatalf("LoadDefault() error: %v", err)
	}
	cat, err := rulecatalog.LoadDefault()
	if err != nil {
		t.Fatalf("LoadDefault catalog error: %v", err)
	}

	catalogDomains := make(map[string]struct{})
	for _, rule := range cat.Rules {
		catalogDomains[rule.Domain] = struct{}{}
	}

	ruleID := regexp.MustCompile(`[A-Z]{1,3}\d+-\d{3}`)
	for _, d := range r.Domains() {
		for _, domain := range d.CatalogDomains {
			if _, ok := catalogDomains[domain]; !ok {
				t.Errorf("domain %q references unknown catalog domain %q", d.ID, domain)
			}
		}
		for _, check := range d.Checks {
			for _, id := range ruleID.FindAllString(check, -1) {
				if !cat.HasRule(id) {
					t.Errorf("domain %q check cites unknown rule ID %q", d.ID, id)
				}
			}
		}
		for _, text := range append([]string{d.ID, d.Sources[0]}, d.Checks...) {
			assertASCII(t, text)
		}
	}
}

func TestRoutePathMatch(t *testing.T) {
	r, err := LoadDefault()
	if err != nil {
		t.Fatal(err)
	}
	got := r.Route([]File{{Path: "native/utils/src/org/jetbrains/kotlin/konan/target/Linker.kt"}})
	if len(got.Domains) == 0 {
		t.Fatal("expected a domain match")
	}
	if got.Domains[0].Domain.ID != "ohos-platform" {
		t.Errorf("first domain = %q, want ohos-platform", got.Domains[0].Domain.ID)
	}
	if len(got.Domains[0].PathHits) == 0 {
		t.Error("expected a path hit")
	}
	if !strings.Contains(got.Rendered, "<domain_knowledge") ||
		!strings.Contains(got.Rendered, "P1-001") {
		t.Errorf("rendered block missing expected content:\n%s", got.Rendered)
	}
	if got.Tokens == 0 || got.Tokens != EstimateTokens(got.Rendered) {
		t.Errorf("Tokens = %d, want %d", got.Tokens, EstimateTokens(got.Rendered))
	}
}

func TestRouteKeywordMatch(t *testing.T) {
	r, err := LoadDefault()
	if err != nil {
		t.Fatal(err)
	}
	got := r.Route([]File{{
		Path:    "native/utils/src/org/jetbrains/kotlin/konan/util/Foo.kt",
		Content: "val target = Family.OHOS\n",
	}})
	if len(got.Domains) == 0 {
		t.Fatal("expected a keyword match")
	}
	if got.Domains[0].Domain.ID != "ohos-platform" {
		t.Errorf("first domain = %q, want ohos-platform", got.Domains[0].Domain.ID)
	}
	if len(got.Domains[0].KeywordHits) == 0 {
		t.Error("expected a keyword hit")
	}
}

func TestRouteNoMatch(t *testing.T) {
	r, err := LoadDefault()
	if err != nil {
		t.Fatal(err)
	}
	got := r.Route([]File{{Path: "README.md", Content: "plain prose"}})
	if got.Enabled != true {
		t.Error("expected Enabled=true")
	}
	if len(got.Domains) != 0 || got.Rendered != "" || got.Tokens != 0 {
		t.Errorf("expected empty result, got %+v", got)
	}
	if got.Summary() != "" {
		t.Errorf("Summary() = %q, want empty", got.Summary())
	}
}

func TestRouteDisabled(t *testing.T) {
	r, err := LoadDefault()
	if err != nil {
		t.Fatal(err)
	}
	r.enabled = false
	got := r.Route([]File{{Path: "native/utils/src/org/jetbrains/kotlin/konan/target/Linker.kt"}})
	if got.Enabled || len(got.Domains) != 0 || got.Rendered != "" {
		t.Errorf("disabled router returned %+v", got)
	}
}

func TestRoutePriorityThenPath(t *testing.T) {
	cfg := Config{
		Schema: Schema,
		Domains: []Domain{
			{ID: "keyword-high", Priority: 99, Keywords: []string{"distinctive-token"}, Checks: []string{"keyword check"}},
			{ID: "path-low", Priority: 1, Paths: []string{"src/**"}, Checks: []string{"path check"}},
			{ID: "path-high", Priority: 99, Paths: []string{"src/**"}, Checks: []string{"path check"}},
		},
	}
	r, err := newRouter(cfg)
	if err != nil {
		t.Fatal(err)
	}
	got := r.Route([]File{{Path: "src/a.go", Content: "distinctive-token"}})
	if len(got.Domains) != 2 {
		t.Fatalf("got %d domains, want 2", len(got.Domains))
	}
	if got.Domains[0].Domain.ID != "path-high" {
		t.Errorf("first domain = %q, want path-high (same priority, path evidence wins)", got.Domains[0].Domain.ID)
	}
	if got.Domains[1].Domain.ID != "keyword-high" {
		t.Errorf("second domain = %q, want keyword-high", got.Domains[1].Domain.ID)
	}
}

func TestRouteWeakKeywordOnlyIgnored(t *testing.T) {
	cfg := Config{
		Schema: Schema,
		Domains: []Domain{
			{ID: "weak", Priority: 50, Keywords: []string{"CMS"}, Checks: []string{"weak check"}},
		},
	}
	r, err := newRouter(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if got := r.Route([]File{{Path: "src/a.kt", Content: "var CMS = 1"}}); len(got.Domains) != 0 {
		t.Errorf("weak keyword-only route selected %s", got.Summary())
	}
}

func TestRouteMaxDomains(t *testing.T) {
	cfg := Config{
		Schema:     Schema,
		MaxDomains: 1,
		Domains: []Domain{
			{ID: "a", Priority: 10, Paths: []string{"**/*.kt"}, Checks: []string{"a"}},
			{ID: "b", Priority: 20, Paths: []string{"**/*.kt"}, Checks: []string{"b"}},
		},
	}
	r, err := newRouter(cfg)
	if err != nil {
		t.Fatal(err)
	}
	got := r.Route([]File{{Path: "a.kt"}})
	if len(got.Domains) != 1 || got.Domains[0].Domain.ID != "b" {
		t.Errorf("got %+v, want only domain b", got.Domains)
	}
}

func TestRouteTokenBudget(t *testing.T) {
	long := strings.Repeat("x", 200)
	cfg := Config{
		Schema:      Schema,
		MaxDomains:  2,
		TokenBudget: 160,
		Domains: []Domain{
			{ID: "a", Priority: 20, Paths: []string{"**/*.kt"}, Checks: []string{long, long}},
			{ID: "b", Priority: 10, Paths: []string{"**/*.kt"}, Checks: []string{long, long}},
		},
	}
	r, err := newRouter(cfg)
	if err != nil {
		t.Fatal(err)
	}
	got := r.Route([]File{{Path: "a.kt"}})
	if got.Tokens > r.TokenBudget() {
		t.Errorf("Tokens = %d, want <= %d", got.Tokens, r.TokenBudget())
	}
	if len(got.Domains) == 0 {
		t.Fatal("expected at least one domain")
	}
	if len(got.Domains[0].Domain.Checks) >= 2 {
		t.Errorf("expected checks to be trimmed, got %d", len(got.Domains[0].Domain.Checks))
	}
	if strings.Contains(got.Rendered, long+long) {
		t.Error("rendered block still contains both oversized checks")
	}
}

func TestRouteDeterministic(t *testing.T) {
	r, err := LoadDefault()
	if err != nil {
		t.Fatal(err)
	}
	files := []File{
		{Path: "libraries/tools/kotlin-gradle-plugin/src/Foo.kt"},
		{Path: "native/utils/src/org/jetbrains/kotlin/konan/target/Linker.kt"},
	}
	forward := r.Route(files)
	reverse := r.Route([]File{files[1], files[0]})
	if forward.Rendered != reverse.Rendered {
		t.Errorf("input order changed route output:\n%s\n---\n%s", forward.Rendered, reverse.Rendered)
	}
}

func TestCanonicalConfig(t *testing.T) {
	r, err := LoadDefault()
	if err != nil {
		t.Fatal(err)
	}
	fields := r.CanonicalConfig()
	if len(fields) == 0 {
		t.Fatal("CanonicalConfig() is empty")
	}
	joined := strings.Join(fields, "\x00")
	for _, want := range []string{"domain-route", "ohos-platform", "P1-001", "token_budget"} {
		if !strings.Contains(joined, want) {
			t.Errorf("CanonicalConfig() missing %q", want)
		}
	}
}

func TestLoadProjectOverlayInRuleJSON(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".opencodereview", "rule.json"), `{
  "rules": [{"path": "**/*.go", "rule": "go rule"}],
  "domain_routes": {
    "domains": [
      {"id": "custom", "priority": 200, "paths": ["**/*.go"], "checks": ["custom check"]}
    ]
  }
}`)

	r, err := Load(dir)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	got := r.Route([]File{{Path: "main.go"}})
	if len(got.Domains) == 0 || got.Domains[0].Domain.ID != "custom" {
		t.Fatalf("got %+v, want custom domain first", got.Domains)
	}
	if len(r.Domains()) != 11 {
		t.Errorf("Domains() = %d, want 11 after overlay", len(r.Domains()))
	}
}

func TestLoadDedicatedRoutesOverrideRuleJSON(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".opencodereview", "rule.json"), `{
  "rules": [],
  "domain_routes": {
    "domains": [{"id": "custom", "paths": ["**/*.go"], "checks": ["from rule.json"]}]
  }
}`)
	writeFile(t, filepath.Join(dir, ".opencodereview", "domain-routes.json"), `{
  "schema": "ocr-domain-routes/v1",
  "domains": [{"id": "custom", "paths": ["**/*.go"], "checks": ["from dedicated file"]}]
}`)

	r, err := Load(dir)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	got := r.Route([]File{{Path: "main.go"}})
	if len(got.Domains) == 0 || got.Domains[0].Domain.Checks[0] != "from dedicated file" {
		t.Fatalf("got %+v, want dedicated file override", got.Domains)
	}
}

func TestLoadDisabledProjectConfig(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".opencodereview", "domain-routes.json"), `{
  "schema": "ocr-domain-routes/v1",
  "enabled": false
}`)
	r, err := Load(dir)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if r.Enabled() {
		t.Fatal("router should be disabled")
	}
	if got := r.Route([]File{{Path: "main.go"}}); got.Rendered != "" {
		t.Errorf("disabled router rendered %q", got.Rendered)
	}
}

func TestLoadNoProjectFiles(t *testing.T) {
	r, err := Load(t.TempDir())
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if !r.Enabled() {
		t.Fatal("router should stay enabled")
	}
}

func TestLoadInvalidProjectConfig(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{"bad schema", `{"schema":"other","domains":[]}`},
		{"empty id", `{"schema":"ocr-domain-routes/v1","domains":[{"id":"","paths":["**/*.go"],"checks":["x"]}]}`},
		{"duplicate id", `{"schema":"ocr-domain-routes/v1","domains":[{"id":"a","paths":["**/*.go"],"checks":["x"]},{"id":"a","paths":["**/*.kt"],"checks":["y"]}]}`},
		{"no triggers", `{"schema":"ocr-domain-routes/v1","domains":[{"id":"a","checks":["x"]}]}`},
		{"no checks", `{"schema":"ocr-domain-routes/v1","domains":[{"id":"a","paths":["**/*.go"]}]}`},
		{"bad glob", `{"schema":"ocr-domain-routes/v1","domains":[{"id":"a","paths":["["],"checks":["x"]}]}`},
		{"negative budget", `{"schema":"ocr-domain-routes/v1","token_budget":-1,"domains":[{"id":"a","paths":["**/*.go"],"checks":["x"]}]}`},
		{"bad json", `{`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			writeFile(t, filepath.Join(dir, ".opencodereview", "domain-routes.json"), tt.body)
			if _, err := Load(dir); err == nil {
				t.Fatal("Load() error = nil, want error")
			}
		})
	}
}

func TestLoadRejectsSymlinkEscape(t *testing.T) {
	dir := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside.json")
	writeFile(t, outside, `{"schema":"ocr-domain-routes/v1","domains":[{"id":"a","paths":["**/*.go"],"checks":["x"]}]}`)
	if err := os.MkdirAll(filepath.Join(dir, ".opencodereview"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(dir, ".opencodereview", "domain-routes.json")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if _, err := Load(dir); err == nil {
		t.Fatal("Load() error = nil for symlink escaping the repo")
	}
}

func TestEmbeddedRoutesAreASCII(t *testing.T) {
	if !json.Valid(defaultRoutesJSON) {
		t.Fatal("embedded routes are not valid JSON")
	}
	assertASCII(t, string(defaultRoutesJSON))
}

func TestPr408PathsRoute(t *testing.T) {
	r, err := LoadDefault()
	if err != nil {
		t.Fatal(err)
	}
	groups := [][]string{
		{
			"kotlin-native/backend.native/compiler/ir/backend.native/src/org/jetbrains/kotlin/backend/konan/llvm/LlvmAttributes.kt",
			"native/utils/src/org/jetbrains/kotlin/konan/target/Family.kt",
			"native/utils/src/org/jetbrains/kotlin/konan/target/HostManager.kt",
			"native/utils/src/org/jetbrains/kotlin/konan/target/KonanTarget.kt",
			"native/utils/src/org/jetbrains/kotlin/konan/target/KonanTargetExtenstions.kt",
			"native/utils/src/org/jetbrains/kotlin/konan/target/Sanitizer.kt",
		},
		{
			"native/utils/src/org/jetbrains/kotlin/konan/HostLibcxxRuntimeLibraries.kt",
			"native/utils/src/org/jetbrains/kotlin/konan/target/ClangArgs.kt",
			"native/utils/src/org/jetbrains/kotlin/konan/target/Configurables.kt",
			"native/utils/src/org/jetbrains/kotlin/konan/target/ConfigurablesImpl.kt",
			"native/utils/src/org/jetbrains/kotlin/konan/target/Linker.kt",
		},
		{
			"native/utils/src/org/jetbrains/kotlin/konan/util/DependencyDownloader.kt",
			"native/utils/src/org/jetbrains/kotlin/konan/util/DependencyProcessor.kt",
		},
		{
			"libraries/tools/kotlin-gradle-plugin/src/common/kotlin/org/jetbrains/kotlin/gradle/plugin/mpp/apple/swiftimport/GenerateSyntheticLinkageImportProject.kt",
			"libraries/tools/kotlin-gradle-plugin/src/common/kotlin/org/jetbrains/kotlin/gradle/plugin/mpp/apple/swiftimport/SwiftImportSetupAction.kt",
		},
	}
	wantFirst := []string{"ohos-platform", "ohos-platform", "build-release", "gradle"}
	for i, paths := range groups {
		files := make([]File, 0, len(paths))
		for _, path := range paths {
			files = append(files, File{Path: path})
		}
		got := r.Route(files)
		if len(got.Domains) == 0 {
			t.Fatalf("group %d: no domain matched", i)
		}
		if got.Domains[0].Domain.ID != wantFirst[i] {
			t.Errorf("group %d first domain = %q, want %q (%s)", i, got.Domains[0].Domain.ID, wantFirst[i], got.Summary())
		}
		if got.Tokens > r.TokenBudget() {
			t.Errorf("group %d tokens = %d, want <= %d", i, got.Tokens, r.TokenBudget())
		}
	}
}

func TestPr447AndPr438PathsRoute(t *testing.T) {
	r, err := LoadDefault()
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		file File
		want string
	}{
		{
			name: "prctl memory manager",
			file: File{Path: "kotlin-native/runtime/src/main/cpp/Memory.cpp", Content: "prctl(PR_SET_VMA_ANON_NAME)"},
			want: "memory",
		},
		{
			name: "runtime dump",
			file: File{Path: "kotlin-native/runtime/src/main/cpp/Runtime.cpp", Content: "DumpMemoryAsync"},
			want: "memory",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := r.Route([]File{tt.file})
			if len(got.Domains) == 0 {
				t.Fatal("expected a domain match")
			}
			if got.Domains[0].Domain.ID != tt.want {
				t.Errorf("first domain = %q, want %q", got.Domains[0].Domain.ID, tt.want)
			}
		})
	}
}

func TestCppAndKotlinRoutes(t *testing.T) {
	r, err := LoadDefault()
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		path string
		want string
	}{
		{"src/foo.cpp", "cpp"},
		{"src/foo.kt", "kotlin"},
	}
	for _, tt := range tests {
		got := r.Route([]File{{Path: tt.path}})
		if len(got.Domains) != 1 || got.Domains[0].Domain.ID != tt.want {
			t.Errorf("Route(%q) = %s, want %s", tt.path, got.Summary(), tt.want)
		}
	}
}

func TestGradleRoute(t *testing.T) {
	r, err := LoadDefault()
	if err != nil {
		t.Fatal(err)
	}
	got := r.Route([]File{{Path: "libraries/tools/kotlin-gradle-plugin/src/common/kotlin/Foo.kt"}})
	if len(got.Domains) == 0 || got.Domains[0].Domain.ID != "gradle" {
		t.Errorf("got %s, want gradle", got.Summary())
	}
}

func TestNilRouter(t *testing.T) {
	var r *Router
	if r.Enabled() || r.MaxDomains() != 0 || r.TokenBudget() != 0 || r.Domains() != nil {
		t.Error("nil router accessors should return zero values")
	}
	if r.CanonicalConfig() != nil {
		t.Error("nil router CanonicalConfig() should be nil")
	}
	if got := r.Route([]File{{Path: "a.kt"}}); got.Enabled || got.Rendered != "" {
		t.Errorf("nil router returned %+v", got)
	}
}

func TestEstimateTokens(t *testing.T) {
	if got := EstimateTokens(""); got != 0 {
		t.Errorf("EstimateTokens(\"\") = %d, want 0", got)
	}
	if got := EstimateTokens("12345"); got != 2 {
		t.Errorf("EstimateTokens(\"12345\") = %d, want 2", got)
	}
}

func TestMergeConfig(t *testing.T) {
	disabled := false
	base := Config{
		Schema:     Schema,
		MaxDomains: 2,
		Domains:    []Domain{{ID: "a", Paths: []string{"**/*.go"}, Checks: []string{"old"}}},
	}
	overlay := Config{
		Enabled:     &disabled,
		MaxDomains:  1,
		TokenBudget: 10,
		Domains:     []Domain{{ID: "a", Paths: []string{"**/*.go"}, Checks: []string{"new"}}, {ID: "b", Paths: []string{"**/*.kt"}, Checks: []string{"b"}}},
	}
	if err := mergeConfig(&base, overlay); err != nil {
		t.Fatal(err)
	}
	if base.Enabled == nil || *base.Enabled {
		t.Error("Enabled was not overridden")
	}
	if base.MaxDomains != 1 || base.TokenBudget != 10 {
		t.Errorf("limits = %d/%d, want 1/10", base.MaxDomains, base.TokenBudget)
	}
	if len(base.Domains) != 2 || base.Domains[0].Checks[0] != "new" {
		t.Errorf("domains = %+v", base.Domains)
	}
}

func TestReadRuleFileOverlayMissingKey(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".opencodereview", "rule.json"), `{"rules":[]}`)
	raw, err := readRuleFileOverlay(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) != 0 {
		t.Errorf("got %q, want empty", raw)
	}
}

func assertASCII(t *testing.T, s string) {
	t.Helper()
	for _, r := range s {
		if r > 127 {
			t.Errorf("non-ASCII rune %q in %q", r, s)
		}
	}
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
