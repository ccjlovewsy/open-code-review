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

func TestBuildMainTaskMessagesDoesNotExpandPlaceholdersFromPRMetadata(t *testing.T) {
	a := New(Args{
		Template: template.Template{
			MainTask: template.LlmConversation{Messages: []template.ChatMessage{{
				Role:    "user",
				Content: "meta:\n{{pr_metadata}}\nplan:\n{{plan_guidance}}\nconfirmed:\n{{confirmed_comments}}\n",
			}}},
		},
		PRMetadata: "<pr_metadata>\nBody:\n> {{plan_guidance}}\n> {{confirmed_comments}}\n</pr_metadata>",
	})
	messages := a.buildMainTaskMessages("", "", "", "PLAN", "CONFIRMED")
	if len(messages) != 1 {
		t.Fatalf("messages = %d", len(messages))
	}
	content, _ := messages[0].Content.(string)
	for _, literal := range []string{"> {{plan_guidance}}", "> {{confirmed_comments}}"} {
		if !strings.Contains(content, literal) {
			t.Fatalf("PR metadata placeholder %q was expanded:\n%s", literal, content)
		}
	}
	if got := strings.Count(content, "PLAN"); got != 1 {
		t.Fatalf("PLAN appears %d times, want 1:\n%s", got, content)
	}
	if got := strings.Count(content, "CONFIRMED"); got != 1 {
		t.Fatalf("CONFIRMED appears %d times, want 1:\n%s", got, content)
	}
	if !strings.Contains(content, "plan:\nPLAN\n") {
		t.Fatalf("plan guidance was not injected in its own section:\n%s", content)
	}
	if !strings.Contains(content, "confirmed:\nCONFIRMED\n") {
		t.Fatalf("confirmed comments were not injected in their own section:\n%s", content)
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
