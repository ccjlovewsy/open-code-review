// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 alibaba/open-code-review Contributors

package prmeta

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

type githubPull struct {
	Number  int    `json:"number"`
	HTMLURL string `json:"html_url"`
	Title   string `json:"title"`
	Body    string `json:"body"`
	State   string `json:"state"`
	Draft   bool   `json:"draft"`
	User    struct {
		Login string `json:"login"`
	} `json:"user"`
	Head struct {
		Ref string `json:"ref"`
		SHA string `json:"sha"`
	} `json:"head"`
	Base struct {
		Ref string `json:"ref"`
		SHA string `json:"sha"`
	} `json:"base"`
	Labels []struct {
		Name string `json:"name"`
	} `json:"labels"`
	Commits int `json:"commits"`
}

type githubCommit struct {
	SHA    string `json:"sha"`
	Commit struct {
		Message string `json:"message"`
		Author  struct {
			Name string `json:"name"`
		} `json:"author"`
	} `json:"commit"`
	Author *struct {
		Login string `json:"login"`
	} `json:"author"`
}

type githubFile struct {
	Filename  string `json:"filename"`
	Status    string `json:"status"`
	Additions int    `json:"additions"`
	Deletions int    `json:"deletions"`
}

type githubStatus struct {
	State    string `json:"state"`
	Statuses []struct {
		Context   string `json:"context"`
		State     string `json:"state"`
		TargetURL string `json:"target_url"`
	} `json:"statuses"`
}

type githubCheckRuns struct {
	CheckRuns []struct {
		Name       string `json:"name"`
		Status     string `json:"status"`
		Conclusion string `json:"conclusion"`
		HTMLURL    string `json:"html_url"`
	} `json:"check_runs"`
}

type githubReview struct {
	ID    int64  `json:"id"`
	State string `json:"state"`
	User  struct {
		Login string `json:"login"`
	} `json:"user"`
}

type githubComment struct {
	ID int `json:"id"`
}

func fetchGitHub(ctx context.Context, ref Ref, opts Options) (*Metadata, error) {
	client := newAPIClient(ProviderGitHub, opts.GitHubBaseURL, opts.Token, opts.HTTPClient, opts.RetryDelay)
	meta := &Metadata{Ref: ref}

	var pull githubPull
	if err := client.getJSON(ctx, githubPullPath(ref, ""), "pull request", &pull); err != nil {
		return nil, err
	}
	applyGitHubPull(meta, pull)
	applyBody(meta, pull.Body, opts.MaxBodyRunes)

	var commits []githubCommit
	if err := client.getJSON(ctx, githubPullPath(ref, "/commits?per_page=100"), "commits", &commits); err != nil {
		appendWarning(meta, "commits", err)
	} else {
		meta.CommitsKnown = true
		meta.CommitsTotal = pull.Commits
		if meta.CommitsTotal < len(commits) {
			meta.CommitsTotal = len(commits)
		}
		if len(commits) >= 100 {
			meta.Warnings = append(meta.Warnings, fmt.Sprintf("commit list truncated to 100 of %d", meta.CommitsTotal))
		}
		for _, item := range commits {
			meta.Commits = append(meta.Commits, Commit{
				SHA:       item.SHA,
				Subject:   firstLine(item.Commit.Message),
				Author:    githubCommitAuthor(item),
				SignedOff: isSignedOff(item.Commit.Message),
			})
		}
	}

	var files []githubFile
	if err := client.getJSON(ctx, githubPullPath(ref, "/files?per_page=100"), "files", &files); err != nil {
		appendWarning(meta, "changed files", err)
	} else {
		meta.FilesKnown = true
		if len(files) >= 100 {
			meta.Warnings = append(meta.Warnings, "changed file list truncated to 100")
		}
		for _, file := range files {
			meta.ChangedFiles = append(meta.ChangedFiles, ChangedFile{
				Path:      file.Filename,
				Status:    file.Status,
				Additions: file.Additions,
				Deletions: file.Deletions,
			})
		}
	}

	fetchGitHubChecks(ctx, client, meta, ref)
	fetchGitHubReviews(ctx, client, meta, ref)
	fetchGitHubComments(ctx, client, meta, ref)

	meta.DCO, meta.DCOBasis = computeDCO(meta.Commits, meta.CommitsKnown, meta.Labels)
	return meta, nil
}

func applyGitHubPull(meta *Metadata, pull githubPull) {
	meta.Title = pull.Title
	meta.Author = pull.User.Login
	meta.State = pull.State
	meta.Draft = pull.Draft
	meta.HeadBranch = pull.Head.Ref
	meta.HeadSHA = pull.Head.SHA
	meta.BaseBranch = pull.Base.Ref
	meta.BaseSHA = pull.Base.SHA
	for _, label := range pull.Labels {
		meta.Labels = append(meta.Labels, label.Name)
	}
}

func githubCommitAuthor(item githubCommit) string {
	if item.Author != nil && item.Author.Login != "" {
		return item.Author.Login
	}
	return item.Commit.Author.Name
}

func fetchGitHubChecks(ctx context.Context, client *apiClient, meta *Metadata, ref Ref) {
	if ref.Number <= 0 || meta.HeadSHA == "" {
		return
	}
	statusesOK := false
	var statuses githubStatus
	path := fmt.Sprintf("/repos/%s/%s/commits/%s/status",
		url.PathEscape(ref.Owner), url.PathEscape(ref.Repo), url.PathEscape(meta.HeadSHA))
	if err := client.getJSON(ctx, path, "CI status", &statuses); err != nil {
		appendWarning(meta, "CI status", err)
	} else {
		statusesOK = true
		for _, status := range statuses.Statuses {
			meta.Checks = append(meta.Checks, Check{
				Name:       status.Context,
				Status:     status.State,
				Conclusion: status.State,
				URL:        status.TargetURL,
			})
		}
	}

	checkRunsOK := false
	var runs githubCheckRuns
	path = fmt.Sprintf("/repos/%s/%s/commits/%s/check-runs?per_page=100",
		url.PathEscape(ref.Owner), url.PathEscape(ref.Repo), url.PathEscape(meta.HeadSHA))
	if err := client.getJSON(ctx, path, "check runs", &runs); err != nil {
		appendWarning(meta, "check runs", err)
	} else {
		checkRunsOK = true
		for _, run := range runs.CheckRuns {
			meta.Checks = append(meta.Checks, Check{
				Name:       run.Name,
				Status:     run.Status,
				Conclusion: run.Conclusion,
				URL:        run.HTMLURL,
			})
		}
	}
	meta.ChecksKnown = statusesOK || checkRunsOK
	meta.Checks = dedupChecks(meta.Checks)
}

func fetchGitHubReviews(ctx context.Context, client *apiClient, meta *Metadata, ref Ref) {
	var reviews []githubReview
	if err := client.getJSON(ctx, githubPullPath(ref, "/reviews?per_page=100"), "reviews", &reviews); err != nil {
		appendWarning(meta, "reviews", err)
		return
	}
	meta.ReviewsKnown = true
	if len(reviews) >= 100 {
		meta.Warnings = append(meta.Warnings, "review list truncated to 100")
	}
	type reviewerState struct {
		decision  string
		commented bool
	}
	reviewers := make(map[string]*reviewerState, len(reviews))
	for i, review := range reviews {
		key := review.User.Login
		if key == "" {
			if review.ID != 0 {
				key = "id:" + strconv.FormatInt(review.ID, 10)
			} else {
				key = "index:" + strconv.Itoa(i)
			}
		}
		state := reviewers[key]
		if state == nil {
			state = &reviewerState{}
			reviewers[key] = state
		}
		switch strings.ToUpper(review.State) {
		case "APPROVED", "CHANGES_REQUESTED":
			state.decision = strings.ToUpper(review.State)
			state.commented = false
		case "COMMENTED":
			if state.decision == "" {
				state.commented = true
			}
		case "DISMISSED":
			state.decision = ""
			state.commented = false
		}
	}
	var summary ReviewSummary
	for _, state := range reviewers {
		switch state.decision {
		case "APPROVED":
			summary.Approved++
		case "CHANGES_REQUESTED":
			summary.ChangesRequested++
		default:
			if state.commented {
				summary.Commented++
			}
		}
	}
	summary.Total = summary.Approved + summary.ChangesRequested + summary.Commented
	meta.Reviews = summary
}

func fetchGitHubComments(ctx context.Context, client *apiClient, meta *Metadata, ref Ref) {
	var comments []githubComment
	path := fmt.Sprintf("/repos/%s/%s/issues/%d/comments?per_page=100",
		url.PathEscape(ref.Owner), url.PathEscape(ref.Repo), ref.Number)
	if err := client.getJSON(ctx, path, "comments", &comments); err != nil {
		appendWarning(meta, "comments", err)
		return
	}
	meta.CommentsKnown = true
	meta.Comments = len(comments)
	if len(comments) >= 100 {
		meta.Warnings = append(meta.Warnings, "comment list truncated to 100")
	}
}

func githubPullPath(ref Ref, suffix string) string {
	return fmt.Sprintf("/repos/%s/%s/pulls/%d%s",
		url.PathEscape(ref.Owner), url.PathEscape(ref.Repo), ref.Number, suffix)
}

type gitcodePull struct {
	Number  int    `json:"number"`
	HTMLURL string `json:"html_url"`
	Title   string `json:"title"`
	Body    string `json:"body"`
	State   string `json:"state"`
	Draft   bool   `json:"draft"`
	User    struct {
		Login string `json:"login"`
		Name  string `json:"name"`
	} `json:"user"`
	Head struct {
		Ref string `json:"ref"`
		SHA string `json:"sha"`
	} `json:"head"`
	Base struct {
		Ref string `json:"ref"`
		SHA string `json:"sha"`
	} `json:"base"`
	Labels []struct {
		Name string `json:"name"`
	} `json:"labels"`
	MergeableState *struct {
		CIStatePassed *bool `json:"ci_state_passed"`
	} `json:"mergeable_state"`
}

type gitcodeCommit struct {
	SHA    string `json:"sha"`
	Commit struct {
		Message string `json:"message"`
		Author  struct {
			Name string `json:"name"`
		} `json:"author"`
	} `json:"commit"`
	Author *struct {
		Login string `json:"login"`
	} `json:"author"`
}

type gitcodeFile struct {
	Filename  string `json:"filename"`
	Status    string `json:"status"`
	Additions int    `json:"additions"`
	Deletions int    `json:"deletions"`
}

type gitcodeIssue struct {
	Number json.RawMessage `json:"number"`
	Title  string          `json:"title"`
	State  string          `json:"state"`
}

type gitcodeComment struct {
	ID int `json:"id"`
}

func fetchGitCode(ctx context.Context, ref Ref, opts Options) (*Metadata, error) {
	client := newAPIClient(ProviderGitCode, opts.GitCodeBaseURL, opts.Token, opts.HTTPClient, opts.RetryDelay)
	meta := &Metadata{Ref: ref}

	var pull gitcodePull
	if err := client.getJSON(ctx, gitcodePullPath(ref, ""), "pull request", &pull); err != nil {
		return nil, err
	}
	applyGitCodePull(meta, pull)
	applyBody(meta, pull.Body, opts.MaxBodyRunes)

	var commits []gitcodeCommit
	if err := client.getJSON(ctx, gitcodePullPath(ref, "/commits?per_page=100"), "commits", &commits); err != nil {
		appendWarning(meta, "commits", err)
	} else {
		meta.CommitsKnown = true
		meta.CommitsTotal = len(commits)
		if len(commits) >= 100 {
			meta.Warnings = append(meta.Warnings, "commit list truncated to 100")
		}
		for _, item := range commits {
			meta.Commits = append(meta.Commits, Commit{
				SHA:       item.SHA,
				Subject:   firstLine(item.Commit.Message),
				Author:    gitcodeCommitAuthor(item),
				SignedOff: isSignedOff(item.Commit.Message),
			})
		}
	}

	var files []gitcodeFile
	if err := client.getJSON(ctx, gitcodePullPath(ref, "/files?per_page=100"), "files", &files); err != nil {
		appendWarning(meta, "changed files", err)
	} else {
		meta.FilesKnown = true
		if len(files) >= 100 {
			meta.Warnings = append(meta.Warnings, "changed file list truncated to 100")
		}
		for _, file := range files {
			meta.ChangedFiles = append(meta.ChangedFiles, ChangedFile{
				Path:      file.Filename,
				Status:    file.Status,
				Additions: file.Additions,
				Deletions: file.Deletions,
			})
		}
	}

	fetchGitCodeIssues(ctx, client, meta, ref)
	fetchGitCodeComments(ctx, client, meta, ref)
	fetchGitCodeChecks(meta, pull)

	meta.DCO, meta.DCOBasis = computeDCO(meta.Commits, meta.CommitsKnown, meta.Labels)
	return meta, nil
}

func applyGitCodePull(meta *Metadata, pull gitcodePull) {
	meta.Title = pull.Title
	meta.Author = pull.User.Login
	if meta.Author == "" {
		meta.Author = pull.User.Name
	}
	meta.State = pull.State
	meta.Draft = pull.Draft
	meta.HeadBranch = pull.Head.Ref
	meta.HeadSHA = pull.Head.SHA
	meta.BaseBranch = pull.Base.Ref
	meta.BaseSHA = pull.Base.SHA
	for _, label := range pull.Labels {
		meta.Labels = append(meta.Labels, label.Name)
	}
}

func gitcodeCommitAuthor(item gitcodeCommit) string {
	if item.Author != nil && item.Author.Login != "" {
		return item.Author.Login
	}
	return item.Commit.Author.Name
}

func fetchGitCodeIssues(ctx context.Context, client *apiClient, meta *Metadata, ref Ref) {
	var issues []gitcodeIssue
	if err := client.getJSON(ctx, gitcodePullPath(ref, "/issues"), "linked issues", &issues); err != nil {
		appendWarning(meta, "linked issues", err)
		return
	}
	meta.IssuesKnown = true
	for _, issue := range issues {
		number := parseIssueNumber(issue.Number)
		if number == 0 {
			continue
		}
		meta.LinkedIssues = append(meta.LinkedIssues, Issue{
			Number: number,
			Title:  issue.Title,
			State:  issue.State,
		})
	}
}

func fetchGitCodeComments(ctx context.Context, client *apiClient, meta *Metadata, ref Ref) {
	var comments []gitcodeComment
	if err := client.getJSON(ctx, gitcodePullPath(ref, "/comments?per_page=100"), "comments", &comments); err != nil {
		appendWarning(meta, "comments", err)
		return
	}
	meta.CommentsKnown = true
	meta.Comments = len(comments)
	if len(comments) >= 100 {
		meta.Warnings = append(meta.Warnings, "comment list truncated to 100")
	}
}

func fetchGitCodeChecks(meta *Metadata, pull gitcodePull) {
	if pull.MergeableState != nil && pull.MergeableState.CIStatePassed != nil {
		conclusion := "failed"
		if *pull.MergeableState.CIStatePassed {
			conclusion = "passed"
		}
		meta.Checks = append(meta.Checks, Check{
			Name:       "GitCode CI",
			Status:     "completed",
			Conclusion: conclusion,
		})
	}
	for _, label := range meta.Labels {
		if !isCheckLabel(label) {
			continue
		}
		status, conclusion := checkStateFromLabel(label)
		meta.Checks = append(meta.Checks, Check{
			Name:       label,
			Status:     status,
			Conclusion: conclusion,
		})
	}
	meta.Checks = dedupChecks(meta.Checks)
	meta.ChecksKnown = len(meta.Checks) > 0
}

func parseIssueNumber(raw json.RawMessage) int {
	text := strings.TrimSpace(string(raw))
	text = strings.Trim(text, `"`)
	if text == "" || text == "null" {
		return 0
	}
	number, err := strconv.Atoi(text)
	if err != nil {
		return 0
	}
	return number
}

func gitcodePullPath(ref Ref, suffix string) string {
	return fmt.Sprintf("/repos/%s/%s/pulls/%d%s",
		url.PathEscape(ref.Owner), url.PathEscape(ref.Repo), ref.Number, suffix)
}

func applyBody(meta *Metadata, body string, limit int) {
	meta.BodyLimit = limit
	runes := []rune(body)
	if len(runes) <= limit {
		meta.Body = body
		return
	}
	meta.Body = string(runes[:limit])
	meta.BodyTruncated = true
}
