// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 alibaba/open-code-review Contributors

package main

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/alibaba/open-code-review/internal/stdout"
)

// apiHostRewriteTransport sends requests for the real provider API hosts to a
// local httptest server while keeping the production default client path.
type apiHostRewriteTransport struct {
	target *url.URL
	base   http.RoundTripper
}

func writeJSONResponse(t *testing.T, w http.ResponseWriter, body string) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write([]byte(body)); err != nil {
		t.Errorf("write response: %v", err)
	}
}

func (t apiHostRewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := req.Clone(req.Context())
	clone.URL.Scheme = t.target.Scheme
	clone.URL.Host = t.target.Host
	return t.base.RoundTrip(clone)
}

func TestLoadPRMetadataProgressGoesToStderr(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "fake-token")

	var pullFetched atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/pulls/7"):
			pullFetched.Store(true)
			writeJSONResponse(t, w, `{
				"number": 7, "title": "Progress test", "body": "B", "state": "open",
				"user": {"login": "alice"},
				"head": {"ref": "feature", "sha": "headsha"},
				"base": {"ref": "main", "sha": "basesha"}
			}`)
		case strings.HasSuffix(r.URL.Path, "/status"), strings.HasSuffix(r.URL.Path, "/check-runs"):
			writeJSONResponse(t, w, `{}`)
		default:
			writeJSONResponse(t, w, `[]`)
		}
	}))
	t.Cleanup(server.Close)

	target, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse test server URL: %v", err)
	}
	oldTransport := http.DefaultTransport
	http.DefaultTransport = apiHostRewriteTransport{target: target, base: oldTransport}
	t.Cleanup(func() { http.DefaultTransport = oldTransport })

	var progress bytes.Buffer
	restore := stdout.Swap(&progress)
	defer restore()

	var metadata string
	var stdoutText string
	stderrText := captureStderr(t, func() {
		stdoutText = captureStdout(t, func() {
			metadata = loadPRMetadata(context.Background(), "https://github.com/acme/widgets/pull/7")
		})
	})

	if !pullFetched.Load() {
		t.Fatal("test setup: PR endpoint was not fetched")
	}
	if !strings.Contains(metadata, "Title: Progress test") {
		t.Fatalf("metadata was not rendered:\n%s", metadata)
	}
	if strings.Contains(progress.String(), "[ocr] PR metadata:") {
		t.Fatalf("PR metadata progress leaked through stdout.Writer():\n%s", progress.String())
	}
	if strings.Contains(stdoutText, "[ocr] PR metadata:") {
		t.Fatalf("PR metadata progress leaked into stdout:\n%s", stdoutText)
	}
	if stdoutText != "" {
		t.Fatalf("stdout must stay empty when only progress is emitted, got:\n%s", stdoutText)
	}
	if !strings.Contains(stderrText, "[ocr] PR metadata: acme/widgets#7") {
		t.Fatalf("PR metadata progress missing from stderr:\n%s", stderrText)
	}
}
