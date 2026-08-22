// 最小 MCP server（T-17）：JSON-RPC 2.0 over stdio，newline-delimited。
// 把 YKC 裁判能力包裝成 agent 可呼叫的工具（真實用家接口）。
// 註：生產可換 mark3labs/mcp-go；此處零依賴、足以實證協定與工具語意。
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"ykc/internal/claimview"
	"ykc/internal/rustutil"
)

type rpcMsg struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   json.RawMessage `json:"error,omitempty"`
}

var tools = []map[string]any{
	{
		"name":        "ykc.check",
		"description": "對專案執行 cargo check，回傳結構化錯誤清單（環境產生，非敘述）",
		"inputSchema": map[string]any{"type": "object", "properties": map[string]any{}},
	},
	{
		"name":        "ykc.verify_claims",
		"description": "把代理的聲明與專案真實狀態做確定性比對（verified/contradicted/unverifiable）",
		"inputSchema": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"claims_path": map[string]any{"type": "string", "description": "claims.json 路徑"},
			},
		},
	},
	{
		"name":        "ykc.trust_status",
		"description": "查詢代理當前的信任等級（T0–T3）與行為分",
		"inputSchema": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"agent_id": map[string]any{"type": "string"},
			},
		},
	},
}

func sendResult(id json.RawMessage, result any) {
	b, _ := json.Marshal(result)
	out, _ := json.Marshal(rpcMsg{JSONRPC: "2.0", ID: id, Result: b})
	fmt.Println(string(out))
}

func toolResult(text string, isErr bool) map[string]any {
	return map[string]any{
		"content": []map[string]any{{"type": "text", "text": text}},
		"isError": isErr,
	}
}

func callTool(dir, name string, args map[string]any) map[string]any {
	p := Project{Dir: dir}
	switch name {
	case "ykc.check":
		ok, errMsg := p.compiles()
		if ok {
			return toolResult("✅ cargo check：0 錯誤", false)
		}
		return toolResult("❌ cargo check 失敗：\n"+rustutil.FirstLine(errMsg), false)

	case "ykc.verify_claims":
		cp, _ := args["claims_path"].(string)
		if cp == "" {
			return toolResult("需要 claims_path 參數", true)
		}
		absCP, err := filepath.Abs(cp)
		if err != nil {
			return toolResult("claims_path 無法解析："+err.Error(), true)
		}
		// 邊界加固：claims 檔必須在專案目錄內（防經 MCP 讀任意檔案）
		if !insideProject(dir, absCP) {
			return toolResult("claims_path 必須在專案目錄內", true)
		}
		doc, err := loadClaims(absCP)
		if err != nil {
			return toolResult("載入 claims 失敗："+err.Error(), true)
		}
		var sb strings.Builder
		for _, c := range doc.Claims {
			v := p.Verify(c)
			fmt.Fprintf(&sb, "%s %s — %s\n", v.Verdict, v.Text, v.Evidence)
		}
		return toolResult(sb.String(), false)

	case "ykc.trust_status":
		agent, _ := args["agent_id"].(string)
		if agent == "" {
			return toolResult("需要 agent_id 參數", true)
		}
		facts := readAll(dir)
		lvl := TrustLevel(claimview.TrustLevel(facts, agent, int(T3)))
		return toolResult(fmt.Sprintf("代理 %s 信任等級 = %s", agent, lvl), false)

	default:
		return toolResult("未知工具: "+name, true)
	}
}

func serveMCP(dir string) {
	sc := bufio.NewScanner(os.Stdin)
	sc.Buffer(make([]byte, 1024*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Bytes()
		var m rpcMsg
		if json.Unmarshal(line, &m) != nil {
			continue
		}
		switch m.Method {
		case "initialize":
			sendResult(m.ID, map[string]any{
				"protocolVersion": "2024-11-05",
				"capabilities":    map[string]any{"tools": map[string]any{}},
				"serverInfo":      map[string]any{"name": "ykc-guard", "version": "0.1.0"},
			})
		case "tools/list":
			sendResult(m.ID, map[string]any{"tools": tools})
		case "tools/call":
			var params struct {
				Name      string         `json:"name"`
				Arguments map[string]any `json:"arguments"`
			}
			_ = json.Unmarshal(m.Params, &params)
			sendResult(m.ID, callTool(dir, params.Name, params.Arguments))
		case "notifications/initialized", "notifications/cancelled":
			// 通知：不回覆
		default:
			if len(m.ID) > 0 {
				sendResult(m.ID, map[string]any{"error": map[string]any{"code": -32601, "message": "method not found: " + m.Method}})
			}
		}
	}
}
