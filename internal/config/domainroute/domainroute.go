// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 alibaba/open-code-review Contributors

// Package domainroute selects domain-specific review knowledge from the
// changed file paths and diff keywords. Routing is configuration-driven and
// deliberately separate from rule text resolution: routes supplement the
// per-language rules with checks that only apply to one compiler subdomain.
package domainroute

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/bmatcuk/doublestar/v4"

	"github.com/alibaba/open-code-review/internal/pathutil"
)

// Schema is the route-table schema this package understands.
const Schema = "ocr-domain-routes/v1"

const (
	// DefaultMaxDomains bounds how many domains one review group receives.
	DefaultMaxDomains = 2
	// DefaultTokenBudget bounds the estimated tokens of the rendered block.
	DefaultTokenBudget = 1200
)

//go:embed routes.json
var defaultRoutesJSON []byte

// Config is the on-disk route table. A project may put the same object under
// the "domain_routes" key of .opencodereview/rule.json or in a dedicated
// .opencodereview/domain-routes.json file.
type Config struct {
	Schema      string   `json:"schema"`
	Enabled     *bool    `json:"enabled,omitempty"`
	MaxDomains  int      `json:"max_domains,omitempty"`
	TokenBudget int      `json:"token_budget,omitempty"`
	Domains     []Domain `json:"domains,omitempty"`
}

// Domain is one review subdomain. CatalogDomains links the route to the
// domain names used by the 1.3 rule catalog so the two mappings cannot drift.
type Domain struct {
	ID             string   `json:"id"`
	CatalogDomains []string `json:"catalog_domains,omitempty"`
	Priority       int      `json:"priority,omitempty"`
	Paths          []string `json:"paths,omitempty"`
	Keywords       []string `json:"keywords,omitempty"`
	Sources        []string `json:"sources,omitempty"`
	Checks         []string `json:"checks"`
}

// File is one changed file considered by the router. Content is optional and
// should be the diff body or file body used for keyword matching.
type File struct {
	Path    string
	Content string
}

// PathHit records one path glob matched a file.
type PathHit struct {
	Path    string
	Pattern string
}

// KeywordHit records one keyword matched a file path or content.
type KeywordHit struct {
	Path    string
	Keyword string
}

// Match is one selected domain with the evidence that selected it.
type Match struct {
	Domain      Domain
	PathHits    []PathHit
	KeywordHits []KeywordHit
	score       int
}

// Result is the deterministic routing outcome for a set of files.
type Result struct {
	Enabled  bool
	Domains  []Match
	Tokens   int
	Rendered string
}

// Router matches configured domains against changed files.
type Router struct {
	enabled     bool
	maxDomains  int
	tokenBudget int
	domains     []Domain
}

// LoadDefault parses the embedded route table.
func LoadDefault() (*Router, error) {
	cfg, err := parseConfig(defaultRoutesJSON, true)
	if err != nil {
		return nil, fmt.Errorf("embedded domain routes: %w", err)
	}
	return newRouter(cfg)
}

// Load parses the embedded route table and overlays project configuration.
// A project may define routes under the "domain_routes" key in
// .opencodereview/rule.json and, with higher precedence, in
// .opencodereview/domain-routes.json. Missing project files are not errors.
func Load(repoDir string) (*Router, error) {
	cfg, err := parseConfig(defaultRoutesJSON, true)
	if err != nil {
		return nil, fmt.Errorf("embedded domain routes: %w", err)
	}
	if repoDir == "" {
		return newRouter(cfg)
	}

	embedded, err := readRuleFileOverlay(repoDir)
	if err != nil {
		return nil, err
	}
	if len(embedded) > 0 {
		overlay, err := parseConfig(embedded, false)
		if err != nil {
			return nil, fmt.Errorf("rule.json domain_routes: %w", err)
		}
		if err := mergeConfig(&cfg, overlay); err != nil {
			return nil, fmt.Errorf("rule.json domain_routes: %w", err)
		}
	}

	data, path, err := readProjectRoutes(repoDir)
	if err != nil {
		return nil, err
	}
	if len(data) > 0 {
		overlay, err := parseConfig(data, false)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		if err := mergeConfig(&cfg, overlay); err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
	}

	return newRouter(cfg)
}

// Enabled reports whether routing is active.
func (r *Router) Enabled() bool {
	return r != nil && r.enabled
}

// MaxDomains returns the configured domain cap.
func (r *Router) MaxDomains() int {
	if r == nil {
		return 0
	}
	return r.maxDomains
}

// TokenBudget returns the configured estimated-token budget.
func (r *Router) TokenBudget() int {
	if r == nil {
		return 0
	}
	return r.tokenBudget
}

// Domains returns a copy of the configured domains.
func (r *Router) Domains() []Domain {
	if r == nil {
		return nil
	}
	return append([]Domain(nil), r.domains...)
}

// CanonicalConfig returns a deterministic field list for the run manifest's
// rule_config_sha256. Route changes alter prompts, so they must invalidate a
// resume just like rule text changes do.
func (r *Router) CanonicalConfig() []string {
	if r == nil {
		return nil
	}
	fields := []string{
		"layer", "domain-route",
		"enabled", strconv.FormatBool(r.enabled),
		"max_domains", strconv.Itoa(r.maxDomains),
		"token_budget", strconv.Itoa(r.tokenBudget),
	}
	for _, d := range r.domains {
		fields = append(fields, "domain", d.ID, "priority", strconv.Itoa(d.Priority))
		for _, p := range d.Paths {
			fields = append(fields, "path", p)
		}
		for _, k := range d.Keywords {
			fields = append(fields, "keyword", k)
		}
		for _, c := range d.Checks {
			fields = append(fields, "check", c)
		}
		for _, c := range d.CatalogDomains {
			fields = append(fields, "catalog_domains", c)
		}
		for _, s := range d.Sources {
			fields = append(fields, "sources", s)
		}
	}
	return fields
}

// Route selects at most the configured number of domains for the given files.
// Path matches outrank keyword-only matches; within each class, domain
// priority and the longest matching glob break ties. The rendered block is
// trimmed until it fits the token budget.
func (r *Router) Route(files []File) Result {
	if r == nil || !r.enabled {
		return Result{}
	}

	matches := make([]Match, 0, len(r.domains))
	for _, d := range r.domains {
		m := Match{Domain: d}
		longestPattern := 0
		strongestKeyword := ""
		for _, f := range files {
			lowerPath := strings.ToLower(f.Path)
			for _, pattern := range d.Paths {
				if matched, _ := doublestar.Match(strings.ToLower(pattern), lowerPath); matched {
					m.PathHits = append(m.PathHits, PathHit{Path: f.Path, Pattern: pattern})
					if len(pattern) > longestPattern {
						longestPattern = len(pattern)
					}
				}
			}
			lowerContent := strings.ToLower(f.Content)
			for _, keyword := range d.Keywords {
				lowerKeyword := strings.ToLower(keyword)
				pathHit := strings.Contains(lowerPath, lowerKeyword)
				contentHit := lowerContent != "" && strings.Contains(lowerContent, lowerKeyword)
				if pathHit || contentHit {
					m.KeywordHits = append(m.KeywordHits, KeywordHit{Path: f.Path, Keyword: keyword})
					if len(keyword) > len(strongestKeyword) {
						strongestKeyword = keyword
					}
				}
			}
		}
		// Keywords are weaker evidence than a path glob. A keyword-only match
		// is admitted only when the hit is distinctive enough to avoid routing
		// a generic token such as CMS or LLVM from unrelated code.
		if len(m.PathHits) == 0 && !isStrongKeyword(strongestKeyword) {
			continue
		}
		if len(m.PathHits) == 0 && len(m.KeywordHits) == 0 {
			continue
		}
		sort.Slice(m.PathHits, func(i, j int) bool {
			if m.PathHits[i].Path != m.PathHits[j].Path {
				return m.PathHits[i].Path < m.PathHits[j].Path
			}
			return m.PathHits[i].Pattern < m.PathHits[j].Pattern
		})
		sort.Slice(m.KeywordHits, func(i, j int) bool {
			if m.KeywordHits[i].Path != m.KeywordHits[j].Path {
				return m.KeywordHits[i].Path < m.KeywordHits[j].Path
			}
			return m.KeywordHits[i].Keyword < m.KeywordHits[j].Keyword
		})
		m.score = matchScore(d.Priority, longestPattern, len(m.PathHits) > 0, strongestKeyword)
		matches = append(matches, m)
	}

	sort.SliceStable(matches, func(i, j int) bool {
		if matches[i].score != matches[j].score {
			return matches[i].score > matches[j].score
		}
		if len(matches[i].PathHits) != len(matches[j].PathHits) {
			return len(matches[i].PathHits) > len(matches[j].PathHits)
		}
		return matches[i].Domain.ID < matches[j].Domain.ID
	})
	if len(matches) > r.maxDomains {
		matches = matches[:r.maxDomains]
	}

	return r.render(matches)
}

// Summary lists the selected domains and their trigger class.
func (r Result) Summary() string {
	if len(r.Domains) == 0 {
		return ""
	}
	parts := make([]string, 0, len(r.Domains))
	for _, m := range r.Domains {
		kind := "keyword"
		switch {
		case len(m.PathHits) > 0 && len(m.KeywordHits) > 0:
			kind = "path+keyword"
		case len(m.PathHits) > 0:
			kind = "path"
		}
		parts = append(parts, m.Domain.ID+"("+kind+")")
	}
	return strings.Join(parts, ", ")
}

// EstimateTokens is a cheap byte-based estimate used only to bound the
// injected block. It intentionally does not depend on a model tokenizer so
// routing stays deterministic offline.
func EstimateTokens(s string) int {
	if s == "" {
		return 0
	}
	return (len(s) + 3) / 4
}

func (r *Router) render(matches []Match) Result {
	selected := append([]Match(nil), matches...)
	for {
		rendered := renderMatches(selected)
		tokens := EstimateTokens(rendered)
		if tokens <= r.tokenBudget {
			return Result{Enabled: true, Domains: selected, Tokens: tokens, Rendered: rendered}
		}
		if len(selected) == 0 {
			return Result{Enabled: true}
		}
		last := len(selected) - 1
		if len(selected[last].Domain.Checks) > 1 {
			selected[last].Domain.Checks = selected[last].Domain.Checks[:len(selected[last].Domain.Checks)-1]
			continue
		}
		selected = selected[:last]
	}
}

func renderMatches(matches []Match) string {
	if len(matches) == 0 {
		return ""
	}
	ids := make([]string, 0, len(matches))
	for _, m := range matches {
		ids = append(ids, m.Domain.ID)
	}

	var sb strings.Builder
	sb.WriteString("<domain_knowledge domains=\"")
	sb.WriteString(strings.Join(ids, ","))
	sb.WriteString("\">\n")
	sb.WriteString("These checks were selected from the changed paths and diff keywords. Apply each domain only to the files that triggered it; they supplement, never replace, the rules above.\n")
	for _, m := range matches {
		sb.WriteString("\n## ")
		sb.WriteString(m.Domain.ID)
		sb.WriteString("\n")
		if len(m.Domain.CatalogDomains) > 0 {
			sb.WriteString("Catalog domains: ")
			sb.WriteString(strings.Join(m.Domain.CatalogDomains, ", "))
			sb.WriteString("\n")
		}
		if len(m.Domain.Sources) > 0 {
			sb.WriteString("Sources: ")
			sb.WriteString(strings.Join(m.Domain.Sources, "; "))
			sb.WriteString("\n")
		}
		sb.WriteString("Triggers: ")
		sb.WriteString(renderTriggers(m))
		sb.WriteString("\n")
		for _, check := range m.Domain.Checks {
			sb.WriteString("- ")
			sb.WriteString(check)
			sb.WriteString("\n")
		}
	}
	sb.WriteString("</domain_knowledge>")
	return sb.String()
}

func renderTriggers(m Match) string {
	var triggers []string
	if len(m.PathHits) > 0 {
		hit := m.PathHits[0]
		triggers = append(triggers, "path `"+hit.Path+"` matched `"+hit.Pattern+"`")
	}
	if len(m.KeywordHits) > 0 {
		hit := m.KeywordHits[0]
		triggers = append(triggers, "keyword `"+hit.Keyword+"` in `"+hit.Path+"`")
	}
	shown := len(m.PathHits)
	if len(m.KeywordHits) > shown {
		shown = len(m.KeywordHits)
	}
	if shown > 1 {
		triggers = append(triggers, fmt.Sprintf("(+%d more)", shown-1))
	}
	return strings.Join(triggers, "; ")
}

func matchScore(priority, longestPattern int, hasPath bool, strongestKeyword string) int {
	if hasPath {
		return priority*1_000_000 + 1_000 + longestPattern
	}
	if strongestKeyword != "" {
		return priority*1_000_000 + len(strongestKeyword)
	}
	return 0
}

func isStrongKeyword(keyword string) bool {
	return len(keyword) >= 6 || strings.ContainsAny(keyword, "._")
}

func parseConfig(data []byte, strict bool) (Config, error) {
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, err
	}
	if cfg.Schema == "" && strict {
		return Config{}, fmt.Errorf("schema = %q, want %q", cfg.Schema, Schema)
	}
	if cfg.Schema != "" && cfg.Schema != Schema {
		return Config{}, fmt.Errorf("schema = %q, want %q", cfg.Schema, Schema)
	}
	if cfg.MaxDomains < 0 {
		return Config{}, errors.New("max_domains must not be negative")
	}
	if cfg.TokenBudget < 0 {
		return Config{}, errors.New("token_budget must not be negative")
	}
	seen := make(map[string]struct{}, len(cfg.Domains))
	for i := range cfg.Domains {
		d := &cfg.Domains[i]
		d.ID = strings.TrimSpace(d.ID)
		if d.ID == "" {
			return Config{}, fmt.Errorf("domain %d has empty id", i)
		}
		if _, duplicate := seen[d.ID]; duplicate {
			return Config{}, fmt.Errorf("duplicate domain ID %q", d.ID)
		}
		seen[d.ID] = struct{}{}
		if d.Priority < 0 {
			return Config{}, fmt.Errorf("domain %q has negative priority", d.ID)
		}
		if len(d.Paths) == 0 && len(d.Keywords) == 0 {
			return Config{}, fmt.Errorf("domain %q has no path or keyword triggers", d.ID)
		}
		for _, pattern := range d.Paths {
			if !doublestar.ValidatePattern(pattern) {
				return Config{}, fmt.Errorf("domain %q has invalid path pattern %q", d.ID, pattern)
			}
		}
		for _, keyword := range d.Keywords {
			if strings.TrimSpace(keyword) == "" {
				return Config{}, fmt.Errorf("domain %q has an empty keyword", d.ID)
			}
		}
		if len(d.Checks) == 0 {
			return Config{}, fmt.Errorf("domain %q has no checks", d.ID)
		}
		for _, check := range d.Checks {
			if strings.TrimSpace(check) == "" {
				return Config{}, fmt.Errorf("domain %q has an empty check", d.ID)
			}
		}
	}
	if strict && len(cfg.Domains) == 0 {
		return Config{}, errors.New("route table has no domains")
	}
	return cfg, nil
}

func newRouter(cfg Config) (*Router, error) {
	enabled := cfg.Enabled == nil || *cfg.Enabled
	if enabled && len(cfg.Domains) == 0 {
		return nil, errors.New("route table has no domains")
	}
	maxDomains := cfg.MaxDomains
	if maxDomains == 0 {
		maxDomains = DefaultMaxDomains
	}
	tokenBudget := cfg.TokenBudget
	if tokenBudget == 0 {
		tokenBudget = DefaultTokenBudget
	}
	return &Router{
		enabled:     enabled,
		maxDomains:  maxDomains,
		tokenBudget: tokenBudget,
		domains:     append([]Domain(nil), cfg.Domains...),
	}, nil
}

func mergeConfig(base *Config, overlay Config) error {
	if overlay.Enabled != nil {
		base.Enabled = overlay.Enabled
	}
	if overlay.MaxDomains != 0 {
		base.MaxDomains = overlay.MaxDomains
	}
	if overlay.TokenBudget != 0 {
		base.TokenBudget = overlay.TokenBudget
	}
	index := make(map[string]int, len(base.Domains))
	for i, d := range base.Domains {
		index[d.ID] = i
	}
	for _, d := range overlay.Domains {
		if i, ok := index[d.ID]; ok {
			base.Domains[i] = d
			continue
		}
		index[d.ID] = len(base.Domains)
		base.Domains = append(base.Domains, d)
	}
	return nil
}

func readRuleFileOverlay(repoDir string) (json.RawMessage, error) {
	path := filepath.Join(repoDir, ".opencodereview", "rule.json")
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read project rule %s: %w", path, err)
	}
	var wrapper struct {
		DomainRoutes json.RawMessage `json:"domain_routes"`
	}
	if err := json.Unmarshal(data, &wrapper); err != nil {
		return nil, fmt.Errorf("parse project rule %s: %w", path, err)
	}
	if len(wrapper.DomainRoutes) == 0 || string(wrapper.DomainRoutes) == "null" {
		return nil, nil
	}
	return wrapper.DomainRoutes, nil
}

func readProjectRoutes(repoDir string) ([]byte, string, error) {
	path := filepath.Join(repoDir, ".opencodereview", "domain-routes.json")
	resolved, err := filepath.EvalSymlinks(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, "", nil
	}
	if err != nil {
		return nil, "", fmt.Errorf("resolve project domain routes %s: %w", path, err)
	}
	root, err := pathutil.CanonicalPath(repoDir)
	if err != nil {
		return nil, "", fmt.Errorf("resolve repo dir %s: %w", repoDir, err)
	}
	if !pathutil.WithinBase(root, resolved) {
		return nil, "", fmt.Errorf("project domain routes escape repo dir: %s", path)
	}
	data, err := os.ReadFile(resolved)
	if err != nil {
		return nil, "", fmt.Errorf("read project domain routes %s: %w", path, err)
	}
	return data, path, nil
}
