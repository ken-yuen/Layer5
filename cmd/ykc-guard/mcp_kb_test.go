package main

import (
	"strings"
	"testing"
)

func mcpText(t *testing.T, result map[string]any) string {
	t.Helper()
	content, ok := result["content"].([]map[string]any)
	if !ok || len(content) != 1 {
		t.Fatalf("unexpected MCP result: %#v", result)
	}
	text, _ := content[0]["text"].(string)
	return text
}

func TestMCPKnowledgeToolsAreAdvertisedAndUsable(t *testing.T) {
	names := map[string]bool{}
	for _, tool := range tools {
		name, _ := tool["name"].(string)
		names[name] = true
	}
	for _, want := range []string{"ykc.kb_search", "ykc.kb_explain"} {
		if !names[want] {
			t.Fatalf("tool list lacks %s", want)
		}
	}

	dir := makeProject(t)
	search := callTool(dir, "ykc.kb_search", map[string]any{
		"query": "E0382",
		"k":     float64(1), // encoding/json decodes MCP numbers as float64.
	})
	if search["isError"] != false {
		t.Fatalf("kb_search error: %#v", search)
	}
	if text := mcpText(t, search); !strings.Contains(text, "KB dataset=") || !strings.Contains(text, "E0382") {
		t.Fatalf("kb_search output missing provenance/card: %s", text)
	}

	explain := callTool(dir, "ykc.kb_explain", map[string]any{"code": "e0382"})
	if explain["isError"] != false {
		t.Fatalf("kb_explain error: %#v", explain)
	}
	if text := mcpText(t, explain); !strings.Contains(text, "精確根原子=kb-") || !strings.Contains(text, "E0382") {
		t.Fatalf("kb_explain output missing exact root/card: %s", text)
	}
}

func TestMCPKnowledgeToolBounds(t *testing.T) {
	dir := makeProject(t)
	badBudget := callTool(dir, "ykc.kb_search", map[string]any{"query": "E0382", "budget": float64(1)})
	if badBudget["isError"] != true || !strings.Contains(mcpText(t, badBudget), "budget 必須介於") {
		t.Fatalf("budget boundary not enforced: %#v", badBudget)
	}
	badCode := callTool(dir, "ykc.kb_explain", map[string]any{"code": "E9999"})
	if badCode["isError"] != true || !strings.Contains(mcpText(t, badCode), "找不到") {
		t.Fatalf("unknown code should be MCP error: %#v", badCode)
	}
}
