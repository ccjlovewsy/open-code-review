// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 alibaba/open-code-review Contributors

package agent

import (
	"strings"
	"testing"

	"github.com/alibaba/open-code-review/internal/config/template"
)

func TestBuildMainTaskMessagesInjectsPRMetadata(t *testing.T) {
	a := New(Args{
		Template: template.Template{
			MainTask: template.LlmConversation{Messages: []template.ChatMessage{{
				Role:    "user",
				Content: "before\n### PR Metadata\n{{pr_metadata}}\n\nafter",
			}}},
		},
		PRMetadata: "<pr_metadata>\nBody:\nhello\n</pr_metadata>",
	})
	messages := a.buildMainTaskMessages("", "", "", "", "")
	if len(messages) != 1 {
		t.Fatalf("messages = %d", len(messages))
	}
	content, _ := messages[0].Content.(string)
	if !strings.Contains(content, "<pr_metadata>\nBody:\nhello\n</pr_metadata>") {
		t.Fatalf("PR metadata not injected:\n%s", content)
	}
	if strings.Contains(content, "{{pr_metadata}}") {
		t.Fatalf("placeholder leaked:\n%s", content)
	}
}

func TestBuildMainTaskMessagesStripsEmptyPRMetadata(t *testing.T) {
	a := New(Args{
		Template: template.Template{
			MainTask: template.LlmConversation{Messages: []template.ChatMessage{{
				Role:    "user",
				Content: "before\n### PR Metadata\n{{pr_metadata}}\n\nafter",
			}}},
		},
	})
	messages := a.buildMainTaskMessages("", "", "", "", "")
	if len(messages) != 1 {
		t.Fatalf("messages = %d", len(messages))
	}
	content, _ := messages[0].Content.(string)
	if content != "before\nafter" {
		t.Fatalf("empty PR metadata was not stripped: %q", content)
	}
}
