// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 alibaba/open-code-review Contributors

package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/alibaba/open-code-review/internal/config/domainroute"
	"github.com/alibaba/open-code-review/internal/stdout"
)

// maxRouteContentBytes bounds the file body used for keyword matching. The
// route command is a diagnostic, so reading the whole file adds no value.
const maxRouteContentBytes = 64 << 10

var rulesRouteRepoDir string

var rulesRouteCmd = &cobra.Command{
	Use:   "route [flags] <file-path>...",
	Short: "Show which domain knowledge applies to one or more file paths",
	Long:  "Show the domain-specific knowledge selected by the routing table for the given file paths. File contents are read when available so keyword triggers can be evaluated.",
	Example: `  ocr rules route internal/config/rules/system_rules.go
  ocr rules route native/utils/src/org/jetbrains/kotlin/konan/target/Linker.kt`,
	Args: minimumArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runRulesRoute(args)
	},
}

func init() {
	addRepoFlag(rulesRouteCmd, &rulesRouteRepoDir)
	rulesCmd.AddCommand(rulesRouteCmd)
}

func runRulesRoute(paths []string) error {
	repo, err := resolveRepoDir(rulesRouteRepoDir)
	if err != nil {
		return err
	}
	router, err := domainroute.Load(repo)
	if err != nil {
		return fmt.Errorf("load domain routes: %w", err)
	}

	out := stdout.Writer()
	fmt.Fprintf(out, "Domain routing: enabled=%t max_domains=%d token_budget=%d\n",
		router.Enabled(), router.MaxDomains(), router.TokenBudget())
	if !router.Enabled() {
		fmt.Fprintln(out, "Domain routing is disabled by project configuration.")
		return nil
	}

	files := make([]domainroute.File, 0, len(paths))
	for _, path := range paths {
		files = append(files, domainroute.File{Path: path, Content: readRouteContent(repo, path)})
		fmt.Fprintf(out, "File: %s\n", path)
	}

	result := router.Route(files)
	fmt.Fprintln(out)
	if len(result.Domains) == 0 {
		fmt.Fprintln(out, "No domain matched.")
		return nil
	}
	for _, m := range result.Domains {
		fmt.Fprintf(out, "- %s\n", m.Domain.ID)
		fmt.Fprintf(out, "  trigger: %s\n", routeTriggerSummary(m))
		if len(m.Domain.Sources) > 0 {
			fmt.Fprintf(out, "  sources: %s\n", strings.Join(m.Domain.Sources, "; "))
		}
		fmt.Fprintf(out, "  checks: %d\n", len(m.Domain.Checks))
	}
	fmt.Fprintf(out, "\nInjected: ~%d tokens\n", result.Tokens)
	return nil
}

func routeTriggerSummary(m domainroute.Match) string {
	parts := make([]string, 0, 2)
	if len(m.PathHits) > 0 {
		hit := m.PathHits[0]
		parts = append(parts, fmt.Sprintf("path %q matched %q (+%d more)", hit.Path, hit.Pattern, len(m.PathHits)-1))
	}
	if len(m.KeywordHits) > 0 {
		hit := m.KeywordHits[0]
		parts = append(parts, fmt.Sprintf("keyword %q in %q (+%d more)", hit.Keyword, hit.Path, len(m.KeywordHits)-1))
	}
	return strings.Join(parts, "; ")
}

func readRouteContent(repo, path string) string {
	full := path
	if !filepath.IsAbs(full) {
		full = filepath.Join(repo, path)
	}
	f, err := os.Open(full)
	if err != nil {
		return ""
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxRouteContentBytes))
	if err != nil {
		return ""
	}
	return string(data)
}
