// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 alibaba/open-code-review Contributors

// Package prmeta loads pull-request metadata from GitHub and GitCode so the
// review prompt can check PR process requirements in addition to the diff.
package prmeta

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	ProviderGitHub  = "github"
	ProviderGitCode = "gitcode"

	DefaultTimeout      = 15 * time.Second
	DefaultMaxBodyRunes = 12000
	DefaultRetryDelay   = 200 * time.Millisecond
	maxResponseBytes    = 10 << 20
	maxCommitSubject    = 200
	maxRenderedCommits  = 50
	maxRenderedFiles    = 20

	// GitCode reports check outcomes as localized label names. Unicode escapes
	// keep the source ASCII while matching the API's actual label text.
	gcLabelSuccess    = "\u6210\u529f"
	gcLabelFailure    = "\u5931\u8d25"
	gcLabelRunning    = "\u8fd0\u884c\u4e2d"
	gcLabelInProgress = "\u8fdb\u884c\u4e2d"
	gcLabelBuild      = "\u7f16\u8bd1"
	gcLabelCheck      = "\u68c0\u67e5"
	gcLabelGate       = "\u95e8\u7981"
)

// Ref identifies one pull request parsed from a user-supplied URL.
type Ref struct {
	Provider string
	Owner    string
	Repo     string
	Number   int
	URL      string
}

// Metadata is the provider-neutral PR snapshot injected into the review prompt.
type Metadata struct {
	Ref           Ref
	Title         string
	Body          string
	BodyTruncated bool
	BodyLimit     int
	Author        string
	State         string
	Draft         bool
	HeadBranch    string
	HeadSHA       string
	BaseBranch    string
	BaseSHA       string
	Labels        []string

	Commits      []Commit
	CommitsKnown bool
	CommitsTotal int

	ChangedFiles []ChangedFile
	FilesKnown   bool

	Checks      []Check
	ChecksKnown bool

	Reviews      ReviewSummary
	ReviewsKnown bool

	Comments      int
	CommentsKnown bool

	LinkedIssues []Issue
	IssuesKnown  bool

	DCO      string
	DCOBasis string

	Warnings []string
}

// Commit is the subset of a PR commit the review prompt needs.
type Commit struct {
	SHA       string
	Subject   string
	Author    string
	SignedOff bool
}

// ChangedFile is the subset of a PR file entry the review prompt needs.
type ChangedFile struct {
	Path      string
	Status    string
	Additions int
	Deletions int
}

// Check is one CI status or check run.
type Check struct {
	Name       string
	Status     string
	Conclusion string
	URL        string
}

// ReviewSummary counts review decisions on a PR.
type ReviewSummary struct {
	Total            int
	Approved         int
	ChangesRequested int
	Commented        int
}

// Issue is one linked issue.
type Issue struct {
	Number int
	Title  string
	State  string
}

// Options controls fetching. Zero values use the defaults.
type Options struct {
	HTTPClient     *http.Client
	Token          string
	GitHubBaseURL  string
	GitCodeBaseURL string
	RetryDelay     time.Duration
	MaxBodyRunes   int
}

// HTTPError is a non-2xx provider response. It deliberately excludes the
// response body and request headers so a token cannot leak through an error.
type HTTPError struct {
	Provider   string
	StatusCode int
	Resource   string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("%s API returned HTTP %d for %s", e.Provider, e.StatusCode, e.Resource)
}

// ParseURL validates a GitHub or GitCode pull-request URL and extracts its
// provider, repository, and number. Only HTTPS URLs on the public hosts are
// accepted; credentials, query strings, and fragments are rejected.
func ParseURL(raw string) (Ref, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return Ref{}, fmt.Errorf("parse PR URL: %w", err)
	}
	if u.Scheme != "https" {
		return Ref{}, fmt.Errorf("PR URL must use https")
	}
	if u.User != nil {
		return Ref{}, fmt.Errorf("PR URL must not embed credentials")
	}
	if u.RawQuery != "" || u.Fragment != "" {
		return Ref{}, fmt.Errorf("PR URL must not contain query parameters or a fragment")
	}

	host := strings.ToLower(u.Hostname())
	parts := splitPath(u.Path)
	if len(parts) != 4 {
		return Ref{}, fmt.Errorf("PR URL must look like https://host/owner/repo/pull/123")
	}
	owner, repo := parts[0], parts[1]
	if owner == "" || repo == "" {
		return Ref{}, fmt.Errorf("PR URL is missing owner or repository")
	}
	number, err := strconv.Atoi(parts[3])
	if err != nil || number <= 0 {
		return Ref{}, fmt.Errorf("PR URL has an invalid pull-request number %q", parts[3])
	}

	ref := Ref{Owner: owner, Repo: repo, Number: number}
	switch host {
	case "github.com", "www.github.com":
		if parts[2] != "pull" {
			return Ref{}, fmt.Errorf("GitHub PR URL must use /pull/<number>")
		}
		ref.Provider = ProviderGitHub
	case "gitcode.com", "www.gitcode.com":
		switch parts[2] {
		case "merge_requests", "pulls", "pull":
		default:
			return Ref{}, fmt.Errorf("GitCode PR URL must use /merge_requests/<number> or /pulls/<number>")
		}
		ref.Provider = ProviderGitCode
	default:
		return Ref{}, fmt.Errorf("unsupported PR host %q (supported: github.com, gitcode.com)", host)
	}
	canonical := url.URL{Scheme: "https", Host: host, Path: "/" + strings.Join(parts, "/")}
	ref.URL = canonical.String()
	return ref, nil
}

func splitPath(path string) []string {
	trimmed := strings.Trim(path, "/")
	if trimmed == "" {
		return nil
	}
	parts := strings.Split(trimmed, "/")
	for i, part := range parts {
		parts[i] = strings.TrimSpace(part)
	}
	return parts
}

// TokenFromEnv returns the first configured token for the provider. Platform
// variables win over the provider-neutral OCR_PR_TOKEN fallback.
func TokenFromEnv(provider string) string {
	switch provider {
	case ProviderGitHub:
		return firstNonEmpty("GITHUB_TOKEN", "GH_TOKEN", "OCR_PR_TOKEN")
	case ProviderGitCode:
		return firstNonEmpty("GITCODE_TOKEN", "GITCODE_ACCESS_TOKEN", "OCR_PR_TOKEN")
	default:
		return ""
	}
}

func firstNonEmpty(names ...string) string {
	for _, name := range names {
		if value := strings.TrimSpace(os.Getenv(name)); value != "" {
			return value
		}
	}
	return ""
}

// Fetch loads the PR snapshot for ref. A failure to load the PR itself is
// returned; failures for auxiliary endpoints are recorded as warnings so a
// partial snapshot still reaches the prompt.
func Fetch(ctx context.Context, ref Ref, opts Options) (*Metadata, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if opts.HTTPClient == nil {
		opts.HTTPClient = &http.Client{Timeout: DefaultTimeout}
	}
	if opts.RetryDelay == 0 {
		opts.RetryDelay = DefaultRetryDelay
	}
	if opts.MaxBodyRunes <= 0 {
		opts.MaxBodyRunes = DefaultMaxBodyRunes
	}
	switch ref.Provider {
	case ProviderGitHub:
		if opts.GitHubBaseURL == "" {
			opts.GitHubBaseURL = "https://api.github.com"
		}
		return fetchGitHub(ctx, ref, opts)
	case ProviderGitCode:
		if opts.GitCodeBaseURL == "" {
			opts.GitCodeBaseURL = "https://api.gitcode.com/api/v5"
		}
		return fetchGitCode(ctx, ref, opts)
	default:
		return nil, fmt.Errorf("unsupported PR provider %q", ref.Provider)
	}
}

// FetchURL parses rawURL and loads its snapshot.
func FetchURL(ctx context.Context, rawURL string, opts Options) (*Metadata, error) {
	ref, err := ParseURL(rawURL)
	if err != nil {
		return nil, err
	}
	return Fetch(ctx, ref, opts)
}

type apiClient struct {
	provider string
	baseURL  string
	token    string
	http     *http.Client
	retry    time.Duration
}

func newAPIClient(provider, baseURL, token string, httpClient *http.Client, retry time.Duration) *apiClient {
	return &apiClient{
		provider: provider,
		baseURL:  strings.TrimRight(baseURL, "/"),
		token:    token,
		http:     clientWithRedirectPolicy(httpClient),
		retry:    retry,
	}
}

// clientWithRedirectPolicy clones the client and strips credentials when a
// redirect leaves the original origin. The clone keeps the caller's client
// untouched, and a caller-supplied CheckRedirect keeps deciding whether the
// redirect may proceed.
func clientWithRedirectPolicy(client *http.Client) *http.Client {
	if client == nil {
		client = &http.Client{Timeout: DefaultTimeout}
	}
	clone := *client
	previous := client.CheckRedirect
	clone.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if previous != nil {
			if err := previous(req, via); err != nil {
				return err
			}
		} else if len(via) >= 10 {
			return fmt.Errorf("stopped after 10 redirects")
		}
		if len(via) > 0 && !sameOrigin(via[0].URL, req.URL) {
			req.Header.Del("Authorization")
			req.Header.Del("PRIVATE-TOKEN")
		}
		return nil
	}
	return &clone
}

func sameOrigin(a, b *url.URL) bool {
	if a == nil || b == nil {
		return false
	}
	if !strings.EqualFold(a.Scheme, b.Scheme) || !strings.EqualFold(a.Hostname(), b.Hostname()) {
		return false
	}
	return effectivePort(a) == effectivePort(b)
}

func effectivePort(u *url.URL) string {
	if port := u.Port(); port != "" {
		return port
	}
	switch strings.ToLower(u.Scheme) {
	case "http":
		return "80"
	case "https":
		return "443"
	default:
		return ""
	}
}

func (c *apiClient) getJSON(ctx context.Context, path, resource string, out any) error {
	var lastErr error
	const attempts = 3
	for attempt := 0; attempt < attempts; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(c.retry * time.Duration(attempt)):
			}
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
		if err != nil {
			return err
		}
		c.setHeaders(req)
		resp, err := c.http.Do(req)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			lastErr = err
			continue
		}
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
		resp.Body.Close()
		if readErr != nil {
			lastErr = readErr
			continue
		}
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			if err := json.Unmarshal(body, out); err != nil {
				return fmt.Errorf("decode %s: %w", resource, err)
			}
			return nil
		}

		httpErr := &HTTPError{Provider: c.provider, StatusCode: resp.StatusCode, Resource: resource}
		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
			lastErr = httpErr
			continue
		}
		return httpErr
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("%s API request failed", c.provider)
	}
	return lastErr
}

func (c *apiClient) setHeaders(req *http.Request) {
	req.Header.Set("User-Agent", "open-code-review")
	req.Header.Set("Accept", "application/json")
	switch c.provider {
	case ProviderGitHub:
		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
		if c.token != "" {
			req.Header.Set("Authorization", "Bearer "+c.token)
		}
	case ProviderGitCode:
		if c.token != "" {
			req.Header.Set("PRIVATE-TOKEN", c.token)
		}
	}
}

func appendWarning(meta *Metadata, resource string, err error) {
	if err == nil {
		return
	}
	meta.Warnings = append(meta.Warnings, fmt.Sprintf("%s unavailable: %v", resource, err))
}

func truncateRunes(s string, limit int) string {
	runes := []rune(s)
	if len(runes) <= limit {
		return s
	}
	return string(runes[:limit])
}

func firstLine(s string) string {
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			return line
		}
	}
	return ""
}

func isSignedOff(message string) bool {
	for _, line := range strings.Split(message, "\n") {
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(line)), "signed-off-by:") {
			return true
		}
	}
	return false
}

func shortSHA(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}

var singleLineSeparators = strings.NewReplacer(
	"\r\n", " ",
	"\r", " ",
	"\n", " ",
	"\v", " ",
	"\f", " ",
	"\u0085", " ",
	"\u2028", " ",
	"\u2029", " ",
)

// sanitizeSingleLine folds provider-controlled text into one line and
// neutralizes block markers, so a crafted value cannot forge a metadata field
// or close the enclosing block. Body keeps its line breaks and is handled
// separately in Render.
func sanitizeSingleLine(s string) string {
	return neutralizeBlockMarkers(singleLineSeparators.Replace(s))
}

func notProvided(s string) string {
	s = strings.TrimSpace(sanitizeSingleLine(s))
	if s == "" {
		return "not provided"
	}
	return s
}

// blockMarkerPattern matches prompt structure tags that provider-controlled
// text must not be able to close or reopen. Matching tolerates case,
// whitespace, and control characters around the slash and tag name.
var blockMarkerPattern = regexp.MustCompile(
	`(?i)<[\s\p{Z}\p{C}]*/?[\s\p{Z}\p{C}]*(?:pr_metadata|user_task|other_changed_files|review_files|domain_knowledge)\b`)

func neutralizeBlockMarkers(s string) string {
	return blockMarkerPattern.ReplaceAllStringFunc(s, func(marker string) string {
		return "<\\" + marker[1:]
	})
}

// quoteBody prefixes every body line with a fixed "> " marker so a crafted
// line cannot forge a metadata field or start a new prompt section. The text
// itself, including blank lines, is preserved.
func quoteBody(body string) string {
	body = neutralizeBlockMarkers(body)
	var b strings.Builder
	b.Grow(len(body) + 2)
	for len(body) > 0 {
		line, sep := splitBodyLine(body)
		b.WriteString("> ")
		b.WriteString(line)
		if sep == "" {
			b.WriteString("\n")
		} else {
			b.WriteString(sep)
		}
		body = body[len(line)+len(sep):]
	}
	return b.String()
}

// splitBodyLine returns the next logical body line and the line separator that
// terminated it. Recognizing CRLF, CR, LF, VT, FF, NEL, U+2028, and U+2029
// keeps a non-LF break from hiding the following line from the quote prefix.
func splitBodyLine(s string) (line, sep string) {
	for i, r := range s {
		switch r {
		case '\r':
			if i+1 < len(s) && s[i+1] == '\n' {
				return s[:i], "\r\n"
			}
			return s[:i], "\r"
		case '\n', '\v', '\f', '\u0085', '\u2028', '\u2029':
			return s[:i], string(r)
		}
	}
	return s, ""
}

// Render formats the snapshot as a <pr_metadata> block. Missing fields are
// spelled out as "not provided" rather than left empty so the model does not
// have to guess whether a value is absent or empty.
func Render(meta *Metadata) string {
	if meta == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString("<pr_metadata")
	fmt.Fprintf(&b, " provider=%q", meta.Ref.Provider)
	fmt.Fprintf(&b, " url=%q>\n", meta.Ref.URL)
	fmt.Fprintf(&b, "Title: %s\n", notProvided(meta.Title))
	fmt.Fprintf(&b, "Author: %s\n", notProvided(meta.Author))
	state := notProvided(meta.State)
	if meta.Draft {
		state += " (draft)"
	}
	fmt.Fprintf(&b, "State: %s\n", state)
	fmt.Fprintf(&b, "Base: %s @ %s\n", notProvided(meta.BaseBranch), notProvided(shortSHA(meta.BaseSHA)))
	fmt.Fprintf(&b, "Head: %s @ %s\n", notProvided(meta.HeadBranch), notProvided(shortSHA(meta.HeadSHA)))
	fmt.Fprintf(&b, "Labels: %s\n", renderLabels(meta.Labels))

	fmt.Fprintf(&b, "Linked issues: %s\n", renderIssues(meta))
	fmt.Fprintf(&b, "Changed files: %s\n", renderChangedFiles(meta))

	b.WriteString("Commits:\n")
	if !meta.CommitsKnown {
		b.WriteString("- not provided\n")
	} else if len(meta.Commits) == 0 {
		b.WriteString("- none reported\n")
	} else {
		for i, commit := range meta.Commits {
			if i >= maxRenderedCommits {
				fmt.Fprintf(&b, "- ... %d more commit(s) omitted\n", meta.CommitsTotal-i)
				break
			}
			signoff := "no"
			if commit.SignedOff {
				signoff = "yes"
			}
			subject := truncateRunes(commit.Subject, maxCommitSubject)
			fmt.Fprintf(&b, "- %s %s (author: %s; Signed-off-by: %s)\n",
				shortSHA(sanitizeSingleLine(commit.SHA)), notProvided(subject), notProvided(commit.Author), signoff)
		}
	}

	b.WriteString("Checks/CI:\n")
	if !meta.ChecksKnown {
		b.WriteString("- not provided\n")
	} else if len(meta.Checks) == 0 {
		b.WriteString("- none reported\n")
	} else {
		for _, check := range meta.Checks {
			fmt.Fprintf(&b, "- %s: %s\n", notProvided(check.Name), renderCheckState(check))
		}
	}

	fmt.Fprintf(&b, "DCO: %s\n", renderDCO(meta))
	fmt.Fprintf(&b, "Reviews: %s\n", renderReviews(meta))
	fmt.Fprintf(&b, "Comments: %s\n", renderComments(meta))

	b.WriteString("Body:\n")
	if strings.TrimSpace(meta.Body) == "" {
		b.WriteString("not provided\n")
	} else {
		b.WriteString(quoteBody(meta.Body))
		if meta.BodyTruncated {
			limit := meta.BodyLimit
			if limit <= 0 {
				limit = DefaultMaxBodyRunes
			}
			fmt.Fprintf(&b, "> [body truncated after %d characters]\n", limit)
		}
	}

	if len(meta.Warnings) > 0 {
		b.WriteString("Metadata warnings:\n")
		for _, warning := range meta.Warnings {
			fmt.Fprintf(&b, "- %s\n", sanitizeSingleLine(warning))
		}
	}
	b.WriteString("</pr_metadata>")
	return b.String()
}

func renderLabels(labels []string) string {
	if len(labels) == 0 {
		return "not provided"
	}
	safe := make([]string, 0, len(labels))
	for _, label := range labels {
		safe = append(safe, sanitizeSingleLine(label))
	}
	return strings.Join(safe, ", ")
}

func renderIssues(meta *Metadata) string {
	if !meta.IssuesKnown {
		return "not provided"
	}
	if len(meta.LinkedIssues) == 0 {
		return "none reported"
	}
	parts := make([]string, 0, len(meta.LinkedIssues))
	for _, issue := range meta.LinkedIssues {
		parts = append(parts, fmt.Sprintf("#%d %s [%s]", issue.Number,
			sanitizeSingleLine(issue.Title), notProvided(issue.State)))
	}
	return strings.Join(parts, "; ")
}

func renderChangedFiles(meta *Metadata) string {
	if !meta.FilesKnown {
		return "not provided"
	}
	if len(meta.ChangedFiles) == 0 {
		return "none reported"
	}
	parts := make([]string, 0, maxRenderedFiles)
	for i, file := range meta.ChangedFiles {
		if i >= maxRenderedFiles {
			parts = append(parts, fmt.Sprintf("... %d more", len(meta.ChangedFiles)-i))
			break
		}
		parts = append(parts, fmt.Sprintf("%s (%s, +%d/-%d)",
			sanitizeSingleLine(file.Path), notProvided(file.Status), file.Additions, file.Deletions))
	}
	return fmt.Sprintf("%d total: %s", len(meta.ChangedFiles), strings.Join(parts, "; "))
}

func renderCheckState(check Check) string {
	status := strings.TrimSpace(sanitizeSingleLine(check.Status))
	conclusion := strings.TrimSpace(sanitizeSingleLine(check.Conclusion))
	switch {
	case conclusion != "" && status != "":
		return status + "/" + conclusion
	case conclusion != "":
		return conclusion
	case status != "":
		return status
	default:
		return "unknown"
	}
}

func renderDCO(meta *Metadata) string {
	if meta.DCO == "" {
		return "not provided"
	}
	state := sanitizeSingleLine(meta.DCO)
	if meta.DCOBasis != "" {
		return state + " (" + sanitizeSingleLine(meta.DCOBasis) + ")"
	}
	return state
}

func renderReviews(meta *Metadata) string {
	if !meta.ReviewsKnown {
		return "not provided"
	}
	return fmt.Sprintf("%d total (approved: %d, changes requested: %d, commented: %d)",
		meta.Reviews.Total, meta.Reviews.Approved, meta.Reviews.ChangesRequested, meta.Reviews.Commented)
}

func renderComments(meta *Metadata) string {
	if !meta.CommentsKnown {
		return "not provided"
	}
	return strconv.Itoa(meta.Comments)
}

func dcoStateFromLabel(label string) string {
	negatedNext := false
	failed, passed := false, false
	for _, token := range labelTokens(label) {
		switch {
		case isNegationToken(token):
			negatedNext = true
			continue
		case isFailureToken(token):
			if !negatedNext {
				failed = true
			}
			negatedNext = false
		case isRunningToken(token):
			negatedNext = false
		case isPassedToken(token):
			if !negatedNext {
				passed = true
			}
			negatedNext = false
		}
	}
	if hasUnnegatedMarker(label, gcLabelFailure) {
		failed = true
	}
	if hasUnnegatedMarker(label, gcLabelSuccess) {
		passed = true
	}
	switch {
	case failed:
		return "failed"
	case passed:
		return "passed"
	default:
		return ""
	}
}

func computeDCO(commits []Commit, commitsKnown bool, labels []string) (string, string) {
	var failedLabel, passedLabel, dcoLabel string
	for _, label := range labels {
		if !strings.Contains(strings.ToLower(label), "dco") {
			continue
		}
		if dcoLabel == "" {
			dcoLabel = label
		}
		switch dcoStateFromLabel(label) {
		case "failed":
			if failedLabel == "" {
				failedLabel = label
			}
		case "passed":
			if passedLabel == "" {
				passedLabel = label
			}
		}
	}
	switch {
	case failedLabel != "":
		return "failed", "label: " + failedLabel
	case passedLabel != "":
		return "passed", "label: " + passedLabel
	case dcoLabel != "":
		return "unknown", "label: " + dcoLabel
	}
	if !commitsKnown {
		return "unknown", "commit list unavailable"
	}
	if len(commits) == 0 {
		return "unknown", "no commits returned"
	}
	signed := 0
	for _, commit := range commits {
		if commit.SignedOff {
			signed++
		}
	}
	if signed == len(commits) {
		return "passed", fmt.Sprintf("all %d commit(s) carry Signed-off-by", len(commits))
	}
	return "unknown", fmt.Sprintf("%d of %d commit(s) carry Signed-off-by", signed, len(commits))
}

func checkStateFromLabel(label string) (string, string) {
	tokens := labelTokens(label)
	negatedNext := false
	failed, running, passed := false, false, false
	for _, token := range tokens {
		switch {
		case isNegationToken(token):
			negatedNext = true
			continue
		case isFailureToken(token):
			if !negatedNext {
				failed = true
			}
		case isRunningToken(token):
			if !negatedNext {
				running = true
			}
		case isPassedToken(token):
			if !negatedNext {
				passed = true
			}
		default:
			continue
		}
		negatedNext = false
	}

	if hasUnnegatedMarker(label, gcLabelFailure) {
		failed = true
	}
	if hasUnnegatedMarker(label, gcLabelInProgress) || hasUnnegatedMarker(label, gcLabelRunning) {
		running = true
	}
	if hasUnnegatedMarker(label, gcLabelSuccess) {
		passed = true
	}

	switch {
	case failed:
		return "completed", "failed"
	case running:
		return "in_progress", "pending"
	case passed:
		return "completed", "passed"
	default:
		return "", ""
	}
}

func isNegationToken(token string) bool {
	switch token {
	case "no", "not", "without":
		return true
	default:
		return false
	}
}

func isFailureToken(token string) bool {
	switch token {
	case "fail", "fails", "failed", "failing", "failure", "failures", "error", "errors", "errored":
		return true
	default:
		return false
	}
}

func isRunningToken(token string) bool {
	switch token {
	case "pending", "running", "progress":
		return true
	default:
		return false
	}
}

func isPassedToken(token string) bool {
	switch token {
	case "pass", "passes", "passed", "passing", "success", "successes", "successful", "succeeded", "succeeds", "ok":
		return true
	default:
		return false
	}
}

// Common Chinese negation prefixes, kept as escapes so the source stays ASCII.
var chineseNegationPrefixes = []string{
	"\u672A",
	"\u65E0",
	"\u4E0D",
	"\u975E",
	"\u6CA1",
	"\u6CA1\u6709",
	"\u672A\u80FD",
	"\u65E0\u6CD5",
	"\u4E0D\u80FD",
}

func hasUnnegatedMarker(label, marker string) bool {
	for offset := 0; offset <= len(label); {
		idx := strings.Index(label[offset:], marker)
		if idx < 0 {
			return false
		}
		pos := offset + idx
		if !hasNegationPrefix(label[:pos]) {
			return true
		}
		offset = pos + len(marker)
	}
	return false
}

func hasNegationPrefix(prefix string) bool {
	for _, neg := range chineseNegationPrefixes {
		if strings.HasSuffix(prefix, neg) {
			return true
		}
	}
	lower := strings.ToLower(prefix)
	for _, neg := range []string{"no", "not", "without"} {
		if !strings.HasSuffix(lower, neg) {
			continue
		}
		start := len(lower) - len(neg)
		if start == 0 || !isASCIIAlnum(lower[start-1]) {
			return true
		}
	}
	return false
}

func isASCIIAlnum(b byte) bool {
	return b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= '0' && b <= '9'
}

func labelTokens(label string) []string {
	return strings.FieldsFunc(strings.ToLower(label), func(r rune) bool {
		return !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'))
	})
}

func isCheckLabel(label string) bool {
	lower := strings.ToLower(label)
	for _, marker := range []string{"dco", "ci", "build", "compile", "test", "lint", "check", gcLabelBuild, gcLabelCheck, gcLabelGate} {
		if strings.Contains(lower, marker) || strings.Contains(label, marker) {
			return true
		}
	}
	return false
}

func dedupChecks(checks []Check) []Check {
	seen := make(map[string]bool, len(checks))
	out := make([]Check, 0, len(checks))
	for _, check := range checks {
		key := check.Name + "\x00" + check.Status + "\x00" + check.Conclusion
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, check)
	}
	return out
}
