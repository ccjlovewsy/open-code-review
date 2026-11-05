// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 alibaba/open-code-review Contributors

//go:build ignore

// Command build-rule-catalog generates the OCR rule catalog from the
// kotlin-pr-review reference tables, the agent-skills reference index, and the
// OCR reverse-mapping document.
//
// The generated catalog is the only allowed source of rule_id values. It is
// checked in so reviews stay reproducible without the upstream repositories;
// regenerate it with:
//
//	go run scripts/build-rule-catalog.go
//
// Use -check to fail when the checked-in catalog is stale, and -verify to
// validate it without access to the upstream sources.
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const (
	catalogSchema         = "ocr-rule-catalog/v1"
	defaultCatalogVersion = "2026-10-30"
	defaultKotlinRoot     = "/tmp/cpf-kmp-ai-review"
	defaultAgentRoot      = "/tmp/agent-skills-ocr-read-20261029"
	defaultOut            = "internal/config/rulecatalog/catalog.json"
	kotlinPRReviewCommit  = "9aafffb80e244dacfde4af54acaa130be3d8a06c"
	agentSkillsCommit     = "0c3dc40885bc6c1be1efd7800d8c94429e4b7ae3"
	kotlinPRReviewURL     = "https://gitcode.com/LPJ_2026/CPF-KMP-CMP-AI.git"
	agentSkillsURL        = "https://gitcode.com/CPF-KMP-CMP/agent-skills"
	ruleLicense           = "Apache-2.0"
	reverseMapName        = "ocr_vs_cpf/\u89c4\u5219\u6620\u5c04\u8868.md"
	legacyReverseMapName  = "\u89c4\u5219\u6620\u5c04\u8868.md"
	ocrRulesHeading       = "### 1.4 OCR"
)

var (
	headingRE = regexp.MustCompile(`^###\s+([A-Z]{1,3}\d+)\s+\x{2014}\s+(.+?)\s*$`)
	rowRE     = regexp.MustCompile(`^\|\s*([A-Z]{1,3}\d+-\d{3})\s*\|\s*(.+?)\s*\|\s*(.+?)\s*\|\s*$`)
	idRE      = regexp.MustCompile(`^([A-Z]+)(\d+)-(\d{3})$`)
	ocrRowRE  = regexp.MustCompile("^\\|\\s*(K\\d+)\\s*\\|\\s*`([^`]+)`[^|]*\\|\\s*(.+?)\\s*\\|\\s*(.+?)\\s*\\|\\s*$")

	severityMap = map[string]string{
		"\u963b\u65ad": "critical",
		"\u4e25\u91cd": "high",
		"\u4e00\u822c": "medium",
	}

	prefixDomain = map[string]string{
		"A":   "architecture",
		"B":   "native-compiler-backend",
		"C":   "c-standard",
		"CXX": "cpp-standard",
		"G":   "gradle-kmp",
		"I":   "interop-platformlibs",
		"J":   "java-standard",
		"KC":  "kotlin-standard",
		"P":   "ohos-platform",
		"R":   "native-runtime",
		"V":   "build-release",
	}

	prefixOrder = []string{"A", "B", "C", "CXX", "G", "I", "J", "KC", "P", "R", "V"}
)

type sourceEntry struct {
	Name    string `json:"name"`
	Repo    string `json:"repo,omitempty"`
	Commit  string `json:"commit"`
	License string `json:"license"`
	Path    string `json:"path"`
}

type ruleSource struct {
	Repo string `json:"repo"`
	Path string `json:"path"`
	Line int    `json:"line"`
}

type appliesTo struct {
	Paths    []string `json:"paths,omitempty"`
	Keywords []string `json:"keywords,omitempty"`
}

type ruleEntry struct {
	ID             string     `json:"id"`
	Namespace      string     `json:"namespace"`
	Prefix         string     `json:"prefix"`
	Title          string     `json:"title"`
	Domain         string     `json:"domain"`
	Severity       string     `json:"severity"`
	SourceSeverity string     `json:"source_severity,omitempty"`
	Check          string     `json:"check"`
	AppliesTo      *appliesTo `json:"applies_to,omitempty"`
	Source         ruleSource `json:"source"`
	Status         string     `json:"status"`
}

type referenceEntry struct {
	ID       string   `json:"id"`
	Module   string   `json:"module"`
	Domain   string   `json:"domain"`
	Facet    string   `json:"facet,omitempty"`
	Audience string   `json:"audience,omitempty"`
	Path     string   `json:"path"`
	Keywords []string `json:"keywords,omitempty"`
}

type namespaceInfo struct {
	Description string            `json:"description"`
	Prefixes    map[string]string `json:"prefixes,omitempty"`
}

type counts struct {
	Rules             int            `json:"rules"`
	CPF               int            `json:"cpf"`
	OCR               int            `json:"ocr"`
	References        int            `json:"references"`
	ReferenceCatalogs int            `json:"reference_catalogs"`
	ByPrefix          map[string]int `json:"by_prefix"`
}

type catalog struct {
	Schema      string                   `json:"schema"`
	Version     string                   `json:"version"`
	GeneratedBy string                   `json:"generated_by"`
	ContentHash string                   `json:"content_hash"`
	Sources     []sourceEntry            `json:"sources"`
	Namespaces  map[string]namespaceInfo `json:"namespaces"`
	Counts      counts                   `json:"counts"`
	Rules       []ruleEntry              `json:"rules"`
	References  []referenceEntry         `json:"references,omitempty"`
}

func main() {
	kotlinRoot := flag.String("kotlin-pr-review", defaultKotlinRoot, "path to the kotlin-pr-review checkout")
	agentRoot := flag.String("agent-skills", defaultAgentRoot, "path to the agent-skills checkout")
	repoRoot := flag.String("repo", ".", "path to the OCR repository")
	out := flag.String("out", defaultOut, "catalog output path")
	version := flag.String("version", defaultCatalogVersion, "catalog knowledge version")
	verify := flag.Bool("verify", false, "validate the existing catalog without regenerating")
	check := flag.Bool("check", false, "fail when the existing catalog is stale")
	allowMissing := flag.Bool("allow-missing-sources", false, "build without the upstream source checkouts")
	flag.Parse()

	if *verify {
		if err := verifyCatalog(*out, *kotlinRoot, *agentRoot, *repoRoot); err != nil {
			fail(err)
		}
		return
	}

	cat, skippedSources, err := buildCatalog(*kotlinRoot, *agentRoot, *repoRoot, *version, *allowMissing)
	if err != nil {
		fail(err)
	}
	partial := len(skippedSources) > 0
	if partial {
		if *check {
			fail(fmt.Errorf("-check requires a complete catalog; skipped sources: %s", strings.Join(skippedSources, "; ")))
		}
		if isDefaultCatalogPath(*out) {
			fail(fmt.Errorf("refusing to write a partial catalog to the default output %s; pass an explicit -out path", defaultOut))
		}
		fmt.Println("WARNING: partial catalog; the following sources were skipped:")
		for _, reason := range skippedSources {
			fmt.Printf("  - %s\n", reason)
		}
	}
	data, err := encodeCatalog(cat)
	if err != nil {
		fail(err)
	}

	if *check {
		current, err := os.ReadFile(*out)
		if err != nil {
			fail(fmt.Errorf("read catalog %s: %w", *out, err))
		}
		if !bytes.Equal(current, data) {
			fail(fmt.Errorf("%s is stale; run: go run scripts/build-rule-catalog.go", *out))
		}
		fmt.Printf("catalog is up to date: %s\n", *out)
		return
	}

	if err := os.MkdirAll(filepath.Dir(*out), 0o755); err != nil {
		fail(fmt.Errorf("create catalog directory: %w", err))
	}
	if err := os.WriteFile(*out, data, 0o644); err != nil {
		fail(fmt.Errorf("write catalog: %w", err))
	}
	if partial {
		fmt.Printf("wrote PARTIAL %s: %d rules (cpf=%d, ocr=%d), %d references from %d catalogs; this catalog is incomplete\n",
			*out, cat.Counts.Rules, cat.Counts.CPF, cat.Counts.OCR, cat.Counts.References, cat.Counts.ReferenceCatalogs)
		return
	}
	fmt.Printf("wrote %s: %d rules (cpf=%d, ocr=%d), %d references from %d catalogs\n",
		*out, cat.Counts.Rules, cat.Counts.CPF, cat.Counts.OCR, cat.Counts.References, cat.Counts.ReferenceCatalogs)
}

func fail(err error) {
	fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
	os.Exit(1)
}

func isDefaultCatalogPath(out string) bool {
	outAbs, err := filepath.Abs(out)
	if err != nil {
		return false
	}
	defaultAbs, err := filepath.Abs(defaultOut)
	if err != nil {
		return false
	}
	return filepath.Clean(outAbs) == filepath.Clean(defaultAbs)
}

func buildCatalog(kotlinRoot, agentRoot, repoRoot, version string, allowMissing bool) (*catalog, []string, error) {
	var skipped []string
	kotlinSkipped := false
	refsDir, err := findReferencesDir(kotlinRoot)
	if err != nil {
		if !allowMissing {
			return nil, nil, err
		}
		if _, statErr := os.Stat(kotlinRoot); statErr == nil {
			return nil, nil, err
		} else if !errors.Is(statErr, os.ErrNotExist) {
			return nil, nil, fmt.Errorf("stat kotlin-pr-review root %s: %w", kotlinRoot, statErr)
		}
		kotlinSkipped = true
		skipped = append(skipped, fmt.Sprintf("kotlin-pr-review root %s does not exist", kotlinRoot))
	}

	var rules []ruleEntry
	if refsDir != "" {
		rules, err = parseKotlinPRReview(kotlinRoot, refsDir)
		if err != nil {
			return nil, nil, err
		}
	}

	ocrRules, err := parseOCRRules(repoRoot)
	if err != nil {
		return nil, nil, err
	}
	rules = append(rules, ocrRules...)
	sortRules(rules)

	references, catalogs, agentSkipped, err := parseAgentSkills(agentRoot, allowMissing)
	if err != nil {
		return nil, nil, err
	}
	if agentSkipped {
		skipped = append(skipped, fmt.Sprintf("agent-skills root %s does not exist", agentRoot))
	}

	cat := &catalog{
		Schema:      catalogSchema,
		Version:     version,
		GeneratedBy: "scripts/build-rule-catalog.go",
		Sources: []sourceEntry{
			{
				Name:    "kotlin-pr-review",
				Repo:    kotlinPRReviewURL,
				Commit:  kotlinPRReviewCommit,
				License: ruleLicense,
				Path:    "skills/kotlin-pr-review/references",
			},
			{
				Name:    "agent-skills",
				Repo:    agentSkillsURL,
				Commit:  agentSkillsCommit,
				License: ruleLicense,
				Path:    "skills",
			},
			{
				Name:    "ocr-reverse-map",
				Commit:  "worktree",
				License: ruleLicense,
				Path:    reverseMapName,
			},
		},
		Namespaces: map[string]namespaceInfo{
			"cpf": {
				Description: "Upstream CPF compiler-review rule IDs. These are the only valid rule_id values for CPF rule mappings.",
				Prefixes:    prefixDomain,
			},
			"ocr": {
				Description: "OCR project rule IDs from the target repository rule set. These are separate from the CPF namespace.",
				Prefixes:    map[string]string{"K": "project-rule"},
			},
		},
		Rules:      rules,
		References: references,
	}
	cat.Counts = countCatalog(cat.Rules, cat.References, catalogs)
	cat.ContentHash = hashContent(cat.Rules, cat.References)

	if err := validateCatalog(cat, false); err != nil {
		return nil, nil, err
	}
	if !kotlinSkipped && cat.Counts.CPF != 207 {
		return nil, nil, fmt.Errorf("expected 207 CPF rules, got %d", cat.Counts.CPF)
	}
	if cat.Counts.OCR != 5 {
		return nil, nil, fmt.Errorf("expected 5 OCR rules, got %d", cat.Counts.OCR)
	}
	return cat, skipped, nil
}

func findReferencesDir(root string) (string, error) {
	for _, candidate := range []string{
		filepath.Join(root, "skills", "kotlin-pr-review", "references"),
		filepath.Join(root, "AIReviewer", "skills", "kotlin-pr-review", "references"),
		filepath.Join(root, "references"),
	} {
		if _, err := os.Stat(filepath.Join(candidate, "comment-format.md")); err == nil {
			return candidate, nil
		}
	}
	if _, err := os.Stat(root); errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("kotlin-pr-review root %s does not exist", root)
	}
	return "", fmt.Errorf("kotlin-pr-review layout is invalid under %s: missing comment-format.md", root)
}

func parseKotlinPRReview(root, refsDir string) ([]ruleEntry, error) {
	custom, err := filepath.Glob(filepath.Join(refsDir, "custom", "*.md"))
	if err != nil {
		return nil, err
	}
	standards, err := filepath.Glob(filepath.Join(refsDir, "*-standard.md"))
	if err != nil {
		return nil, err
	}
	files := append(custom, standards...)
	if len(files) == 0 {
		return nil, fmt.Errorf("no reference tables under %s", refsDir)
	}
	sort.Strings(files)

	var rules []ruleEntry
	seen := map[string]string{}
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", file, err)
		}
		lines := strings.Split(string(data), "\n")
		section := ""
		for i, line := range lines {
			if m := headingRE.FindStringSubmatch(line); m != nil {
				section = strings.TrimSpace(m[2])
				continue
			}
			m := rowRE.FindStringSubmatch(line)
			if m == nil {
				continue
			}
			id := strings.TrimSpace(m[1])
			check := strings.TrimSpace(m[2])
			sourceSeverity := strings.TrimSpace(m[3])
			severity, ok := severityMap[sourceSeverity]
			if !ok {
				return nil, fmt.Errorf("%s:%d: unknown severity %q for %s", file, i+1, sourceSeverity, id)
			}
			prefix := prefixOf(id)
			domain, ok := prefixDomain[prefix]
			if !ok {
				return nil, fmt.Errorf("%s:%d: unknown rule prefix %q for %s", file, i+1, prefix, id)
			}
			if prev, dup := seen[id]; dup {
				return nil, fmt.Errorf("duplicate rule ID %s at %s and %s:%d", id, prev, file, i+1)
			}
			seen[id] = fmt.Sprintf("%s:%d", file, i+1)
			rel, err := filepath.Rel(root, file)
			if err != nil {
				return nil, fmt.Errorf("relative path for %s: %w", file, err)
			}
			rules = append(rules, ruleEntry{
				ID:             id,
				Namespace:      "cpf",
				Prefix:         prefix,
				Title:          section,
				Domain:         domain,
				Severity:       severity,
				SourceSeverity: sourceSeverity,
				Check:          check,
				Source: ruleSource{
					Repo: "kotlin-pr-review",
					Path: filepath.ToSlash(rel),
					Line: i + 1,
				},
				Status: "active",
			})
		}
	}
	return rules, nil
}

func parseOCRRules(repoRoot string) ([]ruleEntry, error) {
	path := filepath.Join(repoRoot, filepath.FromSlash(reverseMapName))
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		legacy := filepath.Join(repoRoot, legacyReverseMapName)
		if legacyData, legacyErr := os.ReadFile(legacy); legacyErr == nil {
			path, data, err = legacy, legacyData, nil
		}
	}
	if err != nil {
		return nil, fmt.Errorf("read rule mapping table %s: %w", path, err)
	}
	lines := strings.Split(string(data), "\n")
	rel, err := filepath.Rel(repoRoot, path)
	if err != nil {
		return nil, fmt.Errorf("relative path for %s: %w", path, err)
	}
	var rules []ruleEntry
	inSection := false
	for i, line := range lines {
		if strings.Contains(line, ocrRulesHeading) {
			inSection = true
			continue
		}
		if !inSection {
			continue
		}
		if strings.HasPrefix(line, "## ") {
			break
		}
		m := ocrRowRE.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		id := strings.TrimSpace(m[1])
		pathPattern := strings.TrimSpace(m[2])
		coverage := strings.TrimSpace(m[3])
		rules = append(rules, ruleEntry{
			ID:        id,
			Namespace: "ocr",
			Prefix:    "K",
			Title:     "OCR project rule " + id,
			Domain:    ocrDomain(pathPattern),
			Severity:  "unspecified",
			Check:     coverage,
			AppliesTo: &appliesTo{Paths: []string{pathPattern}},
			Source: ruleSource{
				Repo: "ocr",
				Path: filepath.ToSlash(rel),
				Line: i + 1,
			},
			Status: "active",
		})
	}
	if len(rules) != 5 {
		return nil, fmt.Errorf("expected 5 OCR rules in %s, got %d", path, len(rules))
	}
	return rules, nil
}

func ocrDomain(pathPattern string) string {
	switch {
	case strings.Contains(pathPattern, "konan.properties"):
		return "build-release"
	case strings.Contains(pathPattern, "runtime"):
		return "native-runtime"
	case strings.Contains(pathPattern, "Bitcode"):
		return "native-compiler-backend"
	case strings.Contains(pathPattern, "scripts"):
		return "build-release"
	default:
		return "general"
	}
}

func parseAgentSkills(root string, allowMissing bool) ([]referenceEntry, int, bool, error) {
	files, err := filepath.Glob(filepath.Join(root, "skills", "*", "references", "catalog.generated.yaml"))
	if err != nil {
		return nil, 0, false, err
	}
	if len(files) == 0 {
		if _, statErr := os.Stat(root); statErr != nil {
			if !errors.Is(statErr, os.ErrNotExist) {
				return nil, 0, false, fmt.Errorf("stat agent-skills root %s: %w", root, statErr)
			}
			if allowMissing {
				return nil, 0, true, nil
			}
			return nil, 0, false, fmt.Errorf("agent-skills root %s does not exist", root)
		}
		return nil, 0, false, fmt.Errorf("no catalog.generated.yaml under %s (root exists)", root)
	}
	sort.Strings(files)

	var references []referenceEntry
	seen := map[string]string{}
	for _, file := range files {
		refs, err := parseReferenceCatalog(file)
		if err != nil {
			return nil, 0, false, err
		}
		for _, ref := range refs {
			if prev, dup := seen[ref.ID]; dup {
				return nil, 0, false, fmt.Errorf("duplicate reference ID %q at %s and %s", ref.ID, prev, file)
			}
			seen[ref.ID] = file
			references = append(references, ref)
		}
	}
	sort.Slice(references, func(i, j int) bool { return references[i].ID < references[j].ID })
	return references, len(files), false, nil
}

func parseReferenceCatalog(path string) ([]referenceEntry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	module := filepath.Base(filepath.Dir(filepath.Dir(path)))
	lines := strings.Split(string(data), "\n")

	var refs []referenceEntry
	var current *referenceEntry
	inKeywords := false
	flush := func() error {
		if current == nil {
			return nil
		}
		if current.ID == "" {
			return fmt.Errorf("%s: reference %s has no id", path, current.Path)
		}
		current.Module = module
		current.Domain = referenceDomain(module)
		refs = append(refs, *current)
		current = nil
		return nil
	}

	for _, line := range lines {
		if strings.HasPrefix(line, "  - path:") {
			if err := flush(); err != nil {
				return nil, err
			}
			raw := unquote(strings.TrimSpace(strings.TrimPrefix(line, "  - path:")))
			current = &referenceEntry{
				Path: filepath.ToSlash(filepath.Join("skills", module, raw)),
			}
			inKeywords = false
			continue
		}
		if current == nil {
			continue
		}
		if strings.HasPrefix(line, "    keywords:") {
			inKeywords = true
			continue
		}
		if inKeywords {
			if strings.HasPrefix(line, "      - ") {
				current.Keywords = append(current.Keywords, unquote(strings.TrimSpace(strings.TrimPrefix(line, "      - "))))
				continue
			}
			inKeywords = false
		}
		switch {
		case strings.HasPrefix(line, "    id:"):
			current.ID = unquote(strings.TrimSpace(strings.TrimPrefix(line, "    id:")))
		case strings.HasPrefix(line, "    facet:"):
			current.Facet = unquote(strings.TrimSpace(strings.TrimPrefix(line, "    facet:")))
		case strings.HasPrefix(line, "    audience:"):
			current.Audience = unquote(strings.TrimSpace(strings.TrimPrefix(line, "    audience:")))
		}
	}
	if err := flush(); err != nil {
		return nil, err
	}
	return refs, nil
}

func referenceDomain(module string) string {
	switch module {
	case "kmp-ohos-platform":
		return "ohos-platform"
	case "kmp-native-compiler-runtime":
		return "native-compiler-backend"
	case "kmp-gc":
		return "native-runtime"
	case "kmp-ffi-akinterop":
		return "interop-platformlibs"
	case "kmp-fusion-render":
		return "render"
	case "kmp-devtools":
		return "devtools"
	case "cpf-kmp-feature-porting":
		return "feature-porting"
	case "cpf-kmp-knowledge-base-management":
		return "knowledge-base"
	case "cpf-kmp-product-repository-management":
		return "repo-management"
	case "two-agent-collaboration":
		return "collaboration"
	default:
		return "general"
	}
}

func unquote(value string) string {
	value = strings.TrimSpace(value)
	if len(value) >= 2 && value[0] == '"' && value[len(value)-1] == '"' {
		if decoded, err := strconv.Unquote(value); err == nil {
			return decoded
		}
	}
	return value
}

func prefixOf(id string) string {
	m := idRE.FindStringSubmatch(id)
	if m == nil {
		return ""
	}
	return m[1]
}

func sortRules(rules []ruleEntry) {
	sort.SliceStable(rules, func(i, j int) bool {
		return ruleSortKey(rules[i].ID) < ruleSortKey(rules[j].ID)
	})
}

func ruleSortKey(id string) string {
	m := idRE.FindStringSubmatch(id)
	if m == nil {
		return "99:" + id
	}
	rank := len(prefixOrder) + 1
	for i, prefix := range prefixOrder {
		if prefix == m[1] {
			rank = i
			break
		}
	}
	chapter, _ := strconv.Atoi(m[2])
	sequence, _ := strconv.Atoi(m[3])
	return fmt.Sprintf("%02d:%05d:%05d", rank, chapter, sequence)
}

func countCatalog(rules []ruleEntry, references []referenceEntry, referenceCatalogs int) counts {
	result := counts{
		Rules:             len(rules),
		References:        len(references),
		ReferenceCatalogs: referenceCatalogs,
		ByPrefix:          map[string]int{},
	}
	for _, rule := range rules {
		result.ByPrefix[rule.Prefix]++
		switch rule.Namespace {
		case "cpf":
			result.CPF++
		case "ocr":
			result.OCR++
		}
	}
	return result
}

func hashContent(rules []ruleEntry, references []referenceEntry) string {
	payload, _ := json.Marshal(struct {
		Rules      []ruleEntry      `json:"rules"`
		References []referenceEntry `json:"references"`
	}{Rules: rules, References: references})
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func encodeCatalog(cat *catalog) ([]byte, error) {
	data, err := json.MarshalIndent(cat, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode catalog: %w", err)
	}
	return append(data, '\n'), nil
}

func verifyCatalog(path, kotlinRoot, agentRoot, repoRoot string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read catalog %s: %w", path, err)
	}
	var cat catalog
	if err := json.Unmarshal(data, &cat); err != nil {
		return fmt.Errorf("parse catalog %s: %w", path, err)
	}
	if err := validateCatalog(&cat, true); err != nil {
		return err
	}

	checked, skipped, skipReasons, err := verifySources(&cat, kotlinRoot, agentRoot, repoRoot)
	if err != nil {
		return err
	}
	for _, reason := range skipReasons {
		fmt.Printf("skipped: %s\n", reason)
	}
	fmt.Printf("catalog ok: %d rules (cpf=%d, ocr=%d), %d references; source lines checked=%d skipped=%d\n",
		cat.Counts.Rules, cat.Counts.CPF, cat.Counts.OCR, cat.Counts.References, checked, skipped)
	return nil
}

func validateCatalog(cat *catalog, strict bool) error {
	if cat.Schema != catalogSchema {
		return fmt.Errorf("catalog schema = %q, want %q", cat.Schema, catalogSchema)
	}
	if cat.Version == "" {
		return errors.New("catalog version is empty")
	}
	if len(cat.Rules) == 0 {
		return errors.New("catalog has no rules")
	}

	seen := map[string]struct{}{}
	for _, rule := range cat.Rules {
		if strings.TrimSpace(rule.ID) == "" {
			return errors.New("catalog rule with empty id")
		}
		if _, dup := seen[rule.ID]; dup {
			return fmt.Errorf("duplicate rule ID %q", rule.ID)
		}
		seen[rule.ID] = struct{}{}
		if rule.Prefix == "" {
			return fmt.Errorf("rule %s has empty prefix", rule.ID)
		}
		if rule.Domain == "" {
			return fmt.Errorf("rule %s has empty domain", rule.ID)
		}
		if rule.Severity == "" {
			return fmt.Errorf("rule %s has empty severity", rule.ID)
		}
		if strings.TrimSpace(rule.Check) == "" {
			return fmt.Errorf("rule %s has empty check", rule.ID)
		}
		if rule.Source.Path == "" || rule.Source.Line <= 0 {
			return fmt.Errorf("rule %s has no source location", rule.ID)
		}
		if strict && rule.Namespace == "cpf" {
			if _, ok := prefixDomain[rule.Prefix]; !ok {
				return fmt.Errorf("rule %s has unknown CPF prefix %q", rule.ID, rule.Prefix)
			}
		}
		if strict && rule.Namespace == "" {
			return fmt.Errorf("rule %s has empty namespace", rule.ID)
		}
	}

	refSeen := map[string]struct{}{}
	for _, ref := range cat.References {
		if ref.ID == "" || ref.Path == "" {
			return fmt.Errorf("reference with empty id or path: %+v", ref)
		}
		if _, dup := refSeen[ref.ID]; dup {
			return fmt.Errorf("duplicate reference ID %q", ref.ID)
		}
		refSeen[ref.ID] = struct{}{}
	}

	if strict {
		want := countCatalog(cat.Rules, cat.References, cat.Counts.ReferenceCatalogs)
		if !countsEqual(cat.Counts, want) {
			return fmt.Errorf("catalog counts are stale: got %+v, want %+v", cat.Counts, want)
		}
		if cat.ContentHash == "" {
			return errors.New("catalog content_hash is empty")
		}
		if got := hashContent(cat.Rules, cat.References); got != cat.ContentHash {
			return fmt.Errorf("catalog content_hash mismatch: got %s, want %s", got, cat.ContentHash)
		}
	}
	return nil
}

func countsEqual(a, b counts) bool {
	if a.Rules != b.Rules || a.CPF != b.CPF || a.OCR != b.OCR ||
		a.References != b.References || a.ReferenceCatalogs != b.ReferenceCatalogs {
		return false
	}
	if len(a.ByPrefix) != len(b.ByPrefix) {
		return false
	}
	for prefix, count := range a.ByPrefix {
		if b.ByPrefix[prefix] != count {
			return false
		}
	}
	return true
}

func verifySources(cat *catalog, kotlinRoot, agentRoot, repoRoot string) (checked, skipped int, skipReasons []string, err error) {
	if _, statErr := os.Stat(repoRoot); statErr != nil {
		return 0, 0, nil, fmt.Errorf("ocr repository root %s: %w", repoRoot, statErr)
	}

	kotlinAvailable := true
	if _, statErr := os.Stat(kotlinRoot); statErr != nil {
		if !errors.Is(statErr, os.ErrNotExist) {
			return 0, 0, nil, fmt.Errorf("stat kotlin-pr-review root %s: %w", kotlinRoot, statErr)
		}
		kotlinAvailable = false
		skipReasons = append(skipReasons, fmt.Sprintf("kotlin-pr-review root %s does not exist", kotlinRoot))
	}

	for _, rule := range cat.Rules {
		switch rule.Source.Repo {
		case "kotlin-pr-review":
			if !kotlinAvailable {
				skipped++
				continue
			}
			path := filepath.Join(kotlinRoot, filepath.FromSlash(rule.Source.Path))
			if err := verifyRuleSource(rule, path); err != nil {
				return checked, skipped, skipReasons, err
			}
			checked++
		case "ocr":
			path, pathErr := resolveReverseMapPath(repoRoot, rule.Source.Path)
			if pathErr != nil {
				return checked, skipped, skipReasons, pathErr
			}
			if err := verifyRuleSource(rule, path); err != nil {
				return checked, skipped, skipReasons, err
			}
			fmt.Printf("checked OCR rule %s at %s:%d\n", rule.ID, path, rule.Source.Line)
			checked++
		default:
			return checked, skipped, skipReasons, fmt.Errorf("rule %s has unknown source repo %q", rule.ID, rule.Source.Repo)
		}
	}

	agentAvailable := true
	if _, statErr := os.Stat(agentRoot); statErr != nil {
		if !errors.Is(statErr, os.ErrNotExist) {
			return checked, skipped, skipReasons, fmt.Errorf("stat agent-skills root %s: %w", agentRoot, statErr)
		}
		agentAvailable = false
		skipReasons = append(skipReasons, fmt.Sprintf("agent-skills root %s does not exist", agentRoot))
	}
	for _, ref := range cat.References {
		if !agentAvailable {
			skipped++
			continue
		}
		path := filepath.Join(agentRoot, filepath.FromSlash(ref.Path))
		if _, readErr := os.ReadFile(path); readErr != nil {
			return checked, skipped, skipReasons, fmt.Errorf("reference %s source file %s is missing or unreadable (agent-skills root exists): %w", ref.ID, path, readErr)
		}
		checked++
	}
	return checked, skipped, skipReasons, nil
}

func resolveReverseMapPath(repoRoot, recorded string) (string, error) {
	recorded = filepath.FromSlash(recorded)
	candidates := []string{filepath.Join(repoRoot, recorded)}
	if recorded == legacyReverseMapName {
		candidates = append(candidates, filepath.Join(repoRoot, filepath.FromSlash(reverseMapName)))
	}
	for _, candidate := range candidates {
		if _, statErr := os.Stat(candidate); statErr == nil {
			return candidate, nil
		} else if !errors.Is(statErr, os.ErrNotExist) {
			return "", fmt.Errorf("stat OCR reverse map %s: %w", candidate, statErr)
		}
	}
	return "", fmt.Errorf("OCR reverse map %q not found under %s (root exists)", recorded, repoRoot)
}

func verifyRuleSource(rule ruleEntry, path string) error {
	data, readErr := os.ReadFile(path)
	if readErr != nil {
		return fmt.Errorf("rule %s source file %s is missing or unreadable: %w", rule.ID, path, readErr)
	}
	lines := strings.Split(string(data), "\n")
	if rule.Source.Line > len(lines) {
		return fmt.Errorf("rule %s source line %d is past EOF in %s", rule.ID, rule.Source.Line, path)
	}
	if !strings.Contains(lines[rule.Source.Line-1], rule.ID) {
		return fmt.Errorf("rule %s not found on %s:%d", rule.ID, path, rule.Source.Line)
	}
	return nil
}
