// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 alibaba/open-code-review Contributors

package prmeta

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestParseURL(t *testing.T) {
	tests := []struct {
		name     string
		raw      string
		provider string
		owner    string
		repo     string
		number   int
		url      string
	}{
		{
			name:     "github",
			raw:      "https://github.com/acme/widgets/pull/7",
			provider: ProviderGitHub,
			owner:    "acme",
			repo:     "widgets",
			number:   7,
			url:      "https://github.com/acme/widgets/pull/7",
		},
		{
			name:     "gitcode merge requests",
			raw:      "https://gitcode.com/CPF-KMP-CMP/kotlin/merge_requests/408",
			provider: ProviderGitCode,
			owner:    "CPF-KMP-CMP",
			repo:     "kotlin",
			number:   408,
			url:      "https://gitcode.com/CPF-KMP-CMP/kotlin/merge_requests/408",
		},
		{
			name:     "gitcode pulls alias",
			raw:      "https://www.gitcode.com/acme/widgets/pulls/12",
			provider: ProviderGitCode,
			owner:    "acme",
			repo:     "widgets",
			number:   12,
			url:      "https://www.gitcode.com/acme/widgets/pulls/12",
		},
		{
			name:     "gitcode pull alias",
			raw:      "https://gitcode.com/acme/widgets/pull/13/",
			provider: ProviderGitCode,
			owner:    "acme",
			repo:     "widgets",
			number:   13,
			url:      "https://gitcode.com/acme/widgets/pull/13",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ref, err := ParseURL(tt.raw)
			if err != nil {
				t.Fatalf("ParseURL() error = %v", err)
			}
			if ref.Provider != tt.provider || ref.Owner != tt.owner || ref.Repo != tt.repo || ref.Number != tt.number {
				t.Fatalf("ParseURL() = %+v", ref)
			}
			if ref.URL != tt.url {
				t.Fatalf("ParseURL() URL = %q, want %q", ref.URL, tt.url)
			}
		})
	}
}

func TestParseURLRejectsInvalidInputs(t *testing.T) {
	tests := []string{
		"http://github.com/acme/widgets/pull/7",
		"https://user:pass@github.com/acme/widgets/pull/7",
		"https://github.com/acme/widgets/pull/7?token=x",
		"https://github.com/acme/widgets/pull/7#fragment",
		"https://example.com/acme/widgets/pull/7",
		"https://github.com/acme/widgets/merge_requests/7",
		"https://gitcode.com/acme/widgets/issues/7",
		"https://github.com/acme/widgets/pull/not-a-number",
		"https://github.com/acme/widgets/pull/0",
		"https://github.com/acme/widgets",
		"not a url",
	}
	for _, raw := range tests {
		t.Run(raw, func(t *testing.T) {
			if _, err := ParseURL(raw); err == nil {
				t.Fatalf("ParseURL(%q) succeeded, want error", raw)
			}
		})
	}
}

func TestTokenFromEnv(t *testing.T) {
	for _, name := range []string{"GITHUB_TOKEN", "GH_TOKEN", "GITCODE_TOKEN", "GITCODE_ACCESS_TOKEN", "OCR_PR_TOKEN"} {
		t.Setenv(name, "")
	}

	t.Setenv("GITHUB_TOKEN", " gh-token ")
	if got := TokenFromEnv(ProviderGitHub); got != "gh-token" {
		t.Fatalf("GitHub token = %q", got)
	}
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "gh-cli")
	if got := TokenFromEnv(ProviderGitHub); got != "gh-cli" {
		t.Fatalf("GitHub fallback token = %q", got)
	}

	t.Setenv("GITCODE_TOKEN", "gc-token")
	if got := TokenFromEnv(ProviderGitCode); got != "gc-token" {
		t.Fatalf("GitCode token = %q", got)
	}
	t.Setenv("GITCODE_TOKEN", "")
	t.Setenv("OCR_PR_TOKEN", "generic")
	if got := TokenFromEnv(ProviderGitHub); got != "gh-cli" {
		t.Fatalf("GitHub neutral fallback = %q", got)
	}
	if got := TokenFromEnv(ProviderGitCode); got != "generic" {
		t.Fatalf("GitCode neutral fallback = %q", got)
	}
	if got := TokenFromEnv("unknown"); got != "" {
		t.Fatalf("unknown provider token = %q", got)
	}
}

func TestFetchGitHub(t *testing.T) {
	var sawAuth atomic.Bool
	var sawVersion atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "Bearer gh-token" {
			sawAuth.Store(true)
		}
		if r.Header.Get("X-GitHub-Api-Version") == "2022-11-28" {
			sawVersion.Store(true)
		}
		switch r.URL.Path {
		case "/repos/acme/widgets/pulls/7":
			writeJSON(t, w, `{
				"number": 7, "title": "Add widget", "body": "## Testing\n- unit tests",
				"state": "open", "draft": false, "commits": 2,
				"user": {"login": "alice"},
				"head": {"ref": "feature", "sha": "headsha"},
				"base": {"ref": "main", "sha": "basesha"},
				"labels": [{"name": "needs-review"}]
			}`)
		case "/repos/acme/widgets/pulls/7/commits":
			writeJSON(t, w, `[
				{"sha": "aaaaaaa111", "commit": {"message": "feat: widget\n\nSigned-off-by: Alice <a@example.com>"}, "author": {"login": "alice"}},
				{"sha": "bbbbbbb222", "commit": {"message": "fix: style"}, "author": {"login": "bob"}}
			]`)
		case "/repos/acme/widgets/pulls/7/files":
			writeJSON(t, w, `[{"filename": "widget.go", "status": "modified", "additions": 12, "deletions": 3}]`)
		case "/repos/acme/widgets/commits/headsha/status":
			writeJSON(t, w, `{"state": "pending", "statuses": [{"context": "ci/build", "state": "success", "target_url": "https://ci/1"}]}`)
		case "/repos/acme/widgets/commits/headsha/check-runs":
			writeJSON(t, w, `{"check_runs": [{"name": "unit", "status": "completed", "conclusion": "failure", "html_url": "https://ci/2"}]}`)
		case "/repos/acme/widgets/pulls/7/reviews":
			writeJSON(t, w, `[{"state": "APPROVED"}, {"state": "CHANGES_REQUESTED"}, {"state": "COMMENTED"}]`)
		case "/repos/acme/widgets/issues/7/comments":
			writeJSON(t, w, `[{"id": 1}, {"id": 2}]`)
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	ref, err := ParseURL("https://github.com/acme/widgets/pull/7")
	if err != nil {
		t.Fatal(err)
	}
	meta, err := Fetch(context.Background(), ref, Options{
		GitHubBaseURL: server.URL,
		Token:         "gh-token",
		RetryDelay:    time.Millisecond,
	})
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
	if !sawAuth.Load() || !sawVersion.Load() {
		t.Fatalf("GitHub headers missing: auth=%v version=%v", sawAuth.Load(), sawVersion.Load())
	}
	if meta.Title != "Add widget" || meta.Author != "alice" || meta.HeadSHA != "headsha" || meta.BaseSHA != "basesha" {
		t.Fatalf("PR fields = %+v", meta)
	}
	if !meta.CommitsKnown || len(meta.Commits) != 2 || meta.CommitsTotal != 2 {
		t.Fatalf("commits = %+v", meta.Commits)
	}
	if !meta.Commits[0].SignedOff || meta.Commits[1].SignedOff {
		t.Fatalf("Signed-off detection = %+v", meta.Commits)
	}
	if !meta.FilesKnown || len(meta.ChangedFiles) != 1 {
		t.Fatalf("files = %+v", meta.ChangedFiles)
	}
	if !meta.ChecksKnown || len(meta.Checks) != 2 {
		t.Fatalf("checks = %+v", meta.Checks)
	}
	if !meta.ReviewsKnown || meta.Reviews.Total != 3 || meta.Reviews.Approved != 1 || meta.Reviews.ChangesRequested != 1 || meta.Reviews.Commented != 1 {
		t.Fatalf("reviews = %+v", meta.Reviews)
	}
	if !meta.CommentsKnown || meta.Comments != 2 {
		t.Fatalf("comments = %d", meta.Comments)
	}
	if meta.DCO != "unknown" || !strings.Contains(meta.DCOBasis, "1 of 2") {
		t.Fatalf("DCO = %q (%s)", meta.DCO, meta.DCOBasis)
	}

	rendered := Render(meta)
	if !strings.Contains(rendered, "<pr_metadata provider=\"github\"") ||
		!strings.Contains(rendered, "Title: Add widget") ||
		!strings.Contains(rendered, "ci/build: success/success") ||
		!strings.Contains(rendered, "unit: completed/failure") ||
		!strings.Contains(rendered, "## Testing") {
		t.Fatalf("rendered metadata missing fields:\n%s", rendered)
	}
	if strings.Contains(rendered, "gh-token") {
		t.Fatal("token leaked into rendered metadata")
	}
}

func TestFetchGitCode(t *testing.T) {
	var sawToken atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("PRIVATE-TOKEN") == "gc-token" {
			sawToken.Store(true)
		}
		switch r.URL.Path {
		case "/repos/CPF-KMP-CMP/kotlin/pulls/408":
			writeJSON(t, w, fmt.Sprintf(`{
				"number": 408, "title": "feat: bootstrap", "body": "## Test\nlocal build",
				"state": "merged", "draft": false,
				"user": {"login": "zhang", "name": "Zhang"},
				"head": {"ref": "feature", "sha": "headsha"},
				"base": {"ref": "develop", "sha": "basesha"},
				"labels": [{"name": "dco%s"}, {"name": "%s%s"}, {"name": "AI"}],
				"mergeable_state": {"ci_state_passed": true}
			}`, gcLabelSuccess, gcLabelBuild, gcLabelSuccess))
		case "/repos/CPF-KMP-CMP/kotlin/pulls/408/commits":
			writeJSON(t, w, `[{"sha": "ccccccc333", "commit": {"message": "feat: bootstrap\n\nSigned-off-by: Zhang <z@example.com>"}, "author": {"login": "zhang"}}]`)
		case "/repos/CPF-KMP-CMP/kotlin/pulls/408/files":
			writeJSON(t, w, `[{"filename": "OAT.xml", "status": "added", "additions": 81, "deletions": 0}]`)
		case "/repos/CPF-KMP-CMP/kotlin/pulls/408/issues":
			writeJSON(t, w, `[{"number": "344", "title": "support bootstrap", "state": "closed"}]`)
		case "/repos/CPF-KMP-CMP/kotlin/pulls/408/comments":
			writeJSON(t, w, `[{"id": 1}]`)
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	ref, err := ParseURL("https://gitcode.com/CPF-KMP-CMP/kotlin/merge_requests/408")
	if err != nil {
		t.Fatal(err)
	}
	meta, err := Fetch(context.Background(), ref, Options{
		GitCodeBaseURL: server.URL,
		Token:          "gc-token",
		RetryDelay:     time.Millisecond,
	})
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
	if !sawToken.Load() {
		t.Fatal("GitCode token header missing")
	}
	if !meta.CommitsKnown || !meta.FilesKnown || !meta.ChecksKnown || !meta.IssuesKnown || !meta.CommentsKnown {
		t.Fatalf("known flags = %+v", meta)
	}
	if meta.DCO != "passed" || !strings.Contains(meta.DCOBasis, "dco") {
		t.Fatalf("DCO = %q (%s)", meta.DCO, meta.DCOBasis)
	}
	if len(meta.LinkedIssues) != 1 || meta.LinkedIssues[0].Number != 344 {
		t.Fatalf("linked issues = %+v", meta.LinkedIssues)
	}
	if meta.ChecksKnown && len(meta.Checks) < 3 {
		t.Fatalf("checks = %+v", meta.Checks)
	}
	rendered := Render(meta)
	for _, want := range []string{
		"GitCode CI: completed/passed",
		"dco" + gcLabelSuccess + ": completed/passed",
		"#344 support bootstrap [closed]",
		"DCO: passed (label: dco" + gcLabelSuccess + ")",
		"Comments: 1",
	} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("rendered metadata missing %q:\n%s", want, rendered)
		}
	}
	if strings.Contains(rendered, "gc-token") {
		t.Fatal("token leaked into rendered metadata")
	}
}

func TestFetchHTTPErrors(t *testing.T) {
	tests := []struct {
		name   string
		status int
	}{
		{name: "unauthorized", status: http.StatusUnauthorized},
		{name: "not found", status: http.StatusNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, "secret upstream detail", tt.status)
			}))
			t.Cleanup(server.Close)

			ref, err := ParseURL("https://github.com/acme/widgets/pull/7")
			if err != nil {
				t.Fatal(err)
			}
			meta, err := Fetch(context.Background(), ref, Options{
				GitHubBaseURL: server.URL,
				Token:         "super-secret-token",
				RetryDelay:    time.Millisecond,
			})
			if err == nil {
				t.Fatal("Fetch() succeeded, want error")
			}
			if meta != nil {
				t.Fatalf("Fetch() metadata = %+v, want nil", meta)
			}
			var httpErr *HTTPError
			if !errors.As(err, &httpErr) || httpErr.StatusCode != tt.status {
				t.Fatalf("error = %v, want HTTPError %d", err, tt.status)
			}
			if strings.Contains(err.Error(), "super-secret-token") || strings.Contains(err.Error(), "secret upstream detail") {
				t.Fatalf("error leaked sensitive data: %v", err)
			}
		})
	}
}

func TestFetchTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		writeJSON(t, w, `{}`)
	}))
	t.Cleanup(server.Close)

	ref, err := ParseURL("https://github.com/acme/widgets/pull/7")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	_, err = Fetch(ctx, ref, Options{
		GitHubBaseURL: server.URL,
		HTTPClient:    &http.Client{Timeout: 20 * time.Millisecond},
		RetryDelay:    time.Millisecond,
	})
	if err == nil {
		t.Fatal("Fetch() succeeded, want timeout")
	}
}

func TestFetchPartialMetadata(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/commits"):
			http.Error(w, "boom", http.StatusInternalServerError)
		case strings.HasSuffix(r.URL.Path, "/reviews"):
			http.Error(w, "forbidden", http.StatusForbidden)
		case strings.HasSuffix(r.URL.Path, "/comments"):
			http.Error(w, "missing", http.StatusNotFound)
		case strings.HasSuffix(r.URL.Path, "/files"):
			writeJSON(t, w, `[]`)
		case strings.HasSuffix(r.URL.Path, "/status"):
			http.Error(w, "missing", http.StatusNotFound)
		case strings.HasSuffix(r.URL.Path, "/check-runs"):
			http.Error(w, "missing", http.StatusNotFound)
		default:
			writeJSON(t, w, `{
				"number": 7, "title": "T", "body": "B", "state": "open",
				"user": {"login": "alice"},
				"head": {"ref": "feature", "sha": "headsha"},
				"base": {"ref": "main", "sha": "basesha"}
			}`)
		}
	}))
	t.Cleanup(server.Close)

	ref, err := ParseURL("https://github.com/acme/widgets/pull/7")
	if err != nil {
		t.Fatal(err)
	}
	meta, err := Fetch(context.Background(), ref, Options{
		GitHubBaseURL: server.URL,
		RetryDelay:    time.Millisecond,
	})
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
	if meta.CommitsKnown || meta.ReviewsKnown || meta.CommentsKnown || meta.ChecksKnown {
		t.Fatalf("known flags should be false: %+v", meta)
	}
	if len(meta.Warnings) < 5 {
		t.Fatalf("warnings = %v", meta.Warnings)
	}
	rendered := Render(meta)
	for _, want := range []string{"Commits:\n- not provided", "Checks/CI:\n- not provided", "Reviews: not provided", "Comments: not provided"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("rendered metadata missing %q:\n%s", want, rendered)
		}
	}
}

func TestFetchRetriesTransientErrors(t *testing.T) {
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/acme/widgets/pulls/7" {
			writeJSON(t, w, `[]`)
			return
		}
		if attempts.Add(1) == 1 {
			http.Error(w, "retry me", http.StatusInternalServerError)
			return
		}
		writeJSON(t, w, `{
			"number": 7, "title": "T", "body": "B", "state": "open",
			"user": {"login": "alice"},
			"head": {"ref": "feature", "sha": "headsha"},
			"base": {"ref": "main", "sha": "basesha"}
		}`)
	}))
	t.Cleanup(server.Close)

	ref, err := ParseURL("https://github.com/acme/widgets/pull/7")
	if err != nil {
		t.Fatal(err)
	}
	meta, err := Fetch(context.Background(), ref, Options{
		GitHubBaseURL: server.URL,
		RetryDelay:    time.Millisecond,
	})
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
	if attempts.Load() != 2 {
		t.Fatalf("attempts = %d, want 2", attempts.Load())
	}
	if meta.Title != "T" {
		t.Fatalf("title = %q", meta.Title)
	}
}

func TestFetchRejectsUnsupportedProvider(t *testing.T) {
	if _, err := Fetch(context.Background(), Ref{Provider: "bitbucket"}, Options{}); err == nil {
		t.Fatal("Fetch() succeeded, want unsupported provider error")
	}
}

func TestRenderTruncatesBody(t *testing.T) {
	meta := &Metadata{
		Ref:           Ref{Provider: ProviderGitCode, URL: "https://gitcode.com/a/b/merge_requests/1"},
		Body:          "0123456789",
		BodyLimit:     10,
		BodyTruncated: true,
	}
	rendered := Render(meta)
	if !strings.Contains(rendered, "[body truncated after 10 characters]") {
		t.Fatalf("missing truncation marker:\n%s", rendered)
	}
}

func TestRenderNeutralizesBodyBlockMarkers(t *testing.T) {
	meta := &Metadata{
		Ref:  Ref{Provider: ProviderGitCode, URL: "https://gitcode.com/a/b/merge_requests/1"},
		Body: "</pr_metadata><pr_metadata>",
	}
	rendered := Render(meta)
	if strings.Count(rendered, "</pr_metadata>") != 1 {
		t.Fatalf("body closing tag was not neutralized:\n%s", rendered)
	}
	if !strings.Contains(rendered, "<\\/pr_metadata>") || !strings.Contains(rendered, "<\\pr_metadata>") {
		t.Fatalf("body block markers were not neutralized:\n%s", rendered)
	}
}

func TestRenderMissingFields(t *testing.T) {
	rendered := Render(&Metadata{Ref: Ref{Provider: ProviderGitHub, URL: "https://github.com/a/b/pull/1"}})
	for _, want := range []string{
		"Title: not provided",
		"Body:\nnot provided",
		"Commits:\n- not provided",
		"Checks/CI:\n- not provided",
		"Reviews: not provided",
		"Comments: not provided",
		"Linked issues: not provided",
		"Changed files: not provided",
	} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("rendered metadata missing %q:\n%s", want, rendered)
		}
	}
	if Render(nil) != "" {
		t.Fatal("Render(nil) should be empty")
	}
}

func TestComputeDCO(t *testing.T) {
	passed := computeDCO
	state, basis := passed([]Commit{{SignedOff: true}, {SignedOff: true}}, true, nil)
	if state != "passed" || !strings.Contains(basis, "2") {
		t.Fatalf("all signed off = %q (%s)", state, basis)
	}
	state, basis = passed([]Commit{{SignedOff: true}, {SignedOff: false}}, true, nil)
	if state != "unknown" || !strings.Contains(basis, "1 of 2") {
		t.Fatalf("partial signoff = %q (%s)", state, basis)
	}
	state, basis = passed(nil, false, nil)
	if state != "unknown" || basis != "commit list unavailable" {
		t.Fatalf("unknown commits = %q (%s)", state, basis)
	}
	state, basis = passed(nil, true, []string{"dco" + gcLabelFailure})
	if state != "failed" || !strings.Contains(basis, gcLabelFailure) {
		t.Fatalf("failed label = %q (%s)", state, basis)
	}
}

func TestCheckConclusionFromLabel(t *testing.T) {
	tests := map[string]string{
		"dco" + gcLabelSuccess:        "passed",
		gcLabelBuild + gcLabelFailure: "failed",
		gcLabelCheck + gcLabelRunning: "pending",
		"build-ok":                    "passed",
	}
	for label, want := range tests {
		if got := checkConclusionFromLabel(label); got != want {
			t.Errorf("checkConclusionFromLabel(%q) = %q, want %q", label, got, want)
		}
	}
}

func writeJSON(t *testing.T, w http.ResponseWriter, body string) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write([]byte(body)); err != nil {
		t.Errorf("write response: %v", err)
	}
}

func TestFetchURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/pulls/7"):
			writeJSON(t, w, `{"number":7,"title":"T","body":"B","state":"open","user":{"login":"a"},"head":{"ref":"f","sha":"h"},"base":{"ref":"b","sha":"s"}}`)
		default:
			writeJSON(t, w, `[]`)
		}
	}))
	t.Cleanup(server.Close)

	meta, err := FetchURL(context.Background(), "https://github.com/acme/widgets/pull/7", Options{
		GitHubBaseURL: server.URL,
		RetryDelay:    time.Millisecond,
	})
	if err != nil {
		t.Fatalf("FetchURL() error = %v", err)
	}
	if meta.Title != "T" {
		t.Fatalf("title = %q", meta.Title)
	}
	if _, err := FetchURL(context.Background(), "not a url", Options{}); err == nil {
		t.Fatal("FetchURL() with invalid URL succeeded")
	}
}

func TestRenderCheckState(t *testing.T) {
	tests := []struct {
		check Check
		want  string
	}{
		{check: Check{Status: "completed", Conclusion: "passed"}, want: "completed/passed"},
		{check: Check{Conclusion: "failure"}, want: "failure"},
		{check: Check{Status: "queued"}, want: "queued"},
		{check: Check{}, want: "unknown"},
	}
	for _, tt := range tests {
		if got := renderCheckState(tt.check); got != tt.want {
			t.Errorf("renderCheckState(%+v) = %q, want %q", tt.check, got, tt.want)
		}
	}
}

func TestCommitAuthorsAndIssueNumbers(t *testing.T) {
	gh := githubCommit{Commit: struct {
		Message string `json:"message"`
		Author  struct {
			Name string `json:"name"`
		} `json:"author"`
	}{Author: struct {
		Name string `json:"name"`
	}{Name: "fallback"}}}
	if got := githubCommitAuthor(gh); got != "fallback" {
		t.Fatalf("githubCommitAuthor() = %q", got)
	}
	gc := gitcodeCommit{}
	gc.Commit.Author.Name = "fallback"
	if got := gitcodeCommitAuthor(gc); got != "fallback" {
		t.Fatalf("gitcodeCommitAuthor() = %q", got)
	}
	for raw, want := range map[string]int{`"12"`: 12, `34`: 34, `"null"`: 0, `""`: 0, `"x"`: 0} {
		if got := parseIssueNumber([]byte(raw)); got != want {
			t.Errorf("parseIssueNumber(%s) = %d, want %d", raw, got, want)
		}
	}
}

func TestRenderChangedFilesLimitAndDCOBasis(t *testing.T) {
	files := make([]ChangedFile, 21)
	for i := range files {
		files[i] = ChangedFile{Path: "f.go", Status: "modified"}
	}
	rendered := Render(&Metadata{
		Ref:          Ref{Provider: ProviderGitHub, URL: "https://github.com/a/b/pull/1"},
		ChangedFiles: files,
		FilesKnown:   true,
		DCO:          "passed",
	})
	if !strings.Contains(rendered, "... 1 more") {
		t.Fatalf("changed file truncation missing:\n%s", rendered)
	}
	if !strings.Contains(rendered, "DCO: passed\n") {
		t.Fatalf("DCO without basis rendered incorrectly:\n%s", rendered)
	}
}
