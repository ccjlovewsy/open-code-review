// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 alibaba/open-code-review Contributors

package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeBadDomainRoutes(t *testing.T, dir string) {
	t.Helper()
	path := filepath.Join(dir, ".opencodereview", "domain-routes.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir domain routes dir: %v", err)
	}
	if err := os.WriteFile(path, []byte("{"), 0o644); err != nil {
		t.Fatalf("write bad domain routes: %v", err)
	}
}

func writeBadRuleCatalog(t *testing.T, dir string) {
	t.Helper()
	path := filepath.Join(dir, ".opencodereview", "rules", "catalog.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir rule catalog dir: %v", err)
	}
	if err := os.WriteFile(path, []byte("{"), 0o644); err != nil {
		t.Fatalf("write bad rule catalog: %v", err)
	}
}

func TestLoadCommonContextSkipsCatalogAndRoutesForDelegate(t *testing.T) {
	dir := initTestGitRepo(t)
	writeBadDomainRoutes(t, dir)
	writeBadRuleCatalog(t, dir)

	cc, err := loadCommonContext(dir, "", "", 0, 0, true, commonContextLoadOptions{})
	if err != nil {
		t.Fatalf("loadCommonContext: %v", err)
	}
	if cc.DomainRoute != nil {
		t.Errorf("DomainRoute = %+v, want nil", cc.DomainRoute)
	}
	if cc.RuleCatalog != nil {
		t.Errorf("RuleCatalog = %+v, want nil", cc.RuleCatalog)
	}
}

func TestReviewNoDomainRoutingBypassesBadRoutes(t *testing.T) {
	freshOCRHome(t)
	dir := initTestGitRepo(t)
	writeBadDomainRoutes(t, dir)

	silenceStdout(t, func() {
		err := executeReviewContext(context.Background(), reviewOptions{
			repoDir:         dir,
			commit:          "HEAD",
			preview:         true,
			noDomainRouting: true,
		})
		if err != nil {
			t.Fatalf("executeReviewContext with --no-domain-routing: %v", err)
		}
	})
}

func TestScanNoDomainRoutingBypassesBadRoutes(t *testing.T) {
	freshOCRHome(t)
	dir := initTestGitRepo(t)
	writeBadDomainRoutes(t, dir)

	silenceStdout(t, func() {
		err := executeScanContext(context.Background(), scanOptions{
			repoDir:         dir,
			preview:         true,
			noDomainRouting: true,
		})
		if err != nil {
			t.Fatalf("executeScanContext with --no-domain-routing: %v", err)
		}
	})
}

func TestReviewEnabledRoutingStillReportsBadRoutes(t *testing.T) {
	freshOCRHome(t)
	dir := initTestGitRepo(t)
	writeBadDomainRoutes(t, dir)

	silenceStdout(t, func() {
		err := executeReviewContext(context.Background(), reviewOptions{
			repoDir: dir,
			commit:  "HEAD",
			preview: true,
		})
		if err == nil || !strings.Contains(err.Error(), "load domain routes") {
			t.Fatalf("err = %v, want a load domain routes error", err)
		}
	})
}

func TestReviewNoDomainRoutingStillReportsBadCatalog(t *testing.T) {
	freshOCRHome(t)
	dir := initTestGitRepo(t)
	writeBadRuleCatalog(t, dir)

	err := executeReviewContext(context.Background(), reviewOptions{
		repoDir:         dir,
		commit:          "HEAD",
		preview:         true,
		noDomainRouting: true,
	})
	if err == nil || !strings.Contains(err.Error(), "load rule catalog") {
		t.Fatalf("err = %v, want a load rule catalog error", err)
	}
}

func TestScanNoDomainRoutingStillReportsBadCatalog(t *testing.T) {
	freshOCRHome(t)
	dir := initTestGitRepo(t)
	writeBadRuleCatalog(t, dir)

	err := executeScanContext(context.Background(), scanOptions{
		repoDir:         dir,
		preview:         true,
		noDomainRouting: true,
	})
	if err == nil || !strings.Contains(err.Error(), "load rule catalog") {
		t.Fatalf("err = %v, want a load rule catalog error", err)
	}
}

func TestDelegateBadCatalogAndRoutesDoNotBlock(t *testing.T) {
	freshOCRHome(t)
	dir := initTestGitRepo(t)
	writeBadDomainRoutes(t, dir)
	writeBadRuleCatalog(t, dir)

	dc, err := loadDelegateContext(delegateOptions{repoDir: dir})
	if err != nil {
		t.Fatalf("loadDelegateContext: %v", err)
	}
	if dc.cc.DomainRoute != nil {
		t.Errorf("DomainRoute = %+v, want nil", dc.cc.DomainRoute)
	}
	if dc.cc.RuleCatalog != nil {
		t.Errorf("RuleCatalog = %+v, want nil", dc.cc.RuleCatalog)
	}
}

func TestRunRulesRouteReportsBadRoutes(t *testing.T) {
	dir := initTestGitRepo(t)
	writeBadDomainRoutes(t, dir)

	oldRepo := rulesRouteRepoDir
	rulesRouteRepoDir = dir
	t.Cleanup(func() { rulesRouteRepoDir = oldRepo })

	err := runRulesRoute([]string{"README.md"})
	if err == nil || !strings.Contains(err.Error(), "load domain routes") {
		t.Fatalf("err = %v, want a load domain routes error", err)
	}
}
