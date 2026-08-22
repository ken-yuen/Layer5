// 最小 MCP server（T-17）：JSON-RPC 2.0 over stdio，newline-delimited。
// 把 YKC 裁判能力包裝成 agent 可呼叫的工具（真實用家接口）。
// 註：生產可換 mark3labs/mcp-go；此處零依賴、足以實證協定與工具語意。
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"ykc/internal/borrow"
	"ykc/internal/claimview"
	"ykc/internal/kb"
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
	{
		"name":        "ykc.borrow_rules",
		"description": "Rust 借用規則幾何卡：兩條幾何法則 + E01–E10 修法菜單（把開放式除錯變成封閉選擇題）。可按 rustc 錯誤碼過濾（如 E0502）",
		"inputSchema": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"code": map[string]any{"type": "string", "description": "rustc 錯誤碼（可選, 如 E0502; 省略=全卡）"},
			},
		},
	},
	{
		"name":        "ykc.borrow_explain",
		"description": "L5 借用幾何解釋：分析 .cl 最小樣例（簡化借用模型語言）, 回傳區間拓撲+代數事實+修法。解釋非判定, 判定以 rustc 為準。cl_source 上限 8KB",
		"inputSchema": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"cl_source": map[string]any{"type": "string", "description": ".cl 源碼（語法見 l5/chordlaw/README.md）"},
			},
		},
	},
	{
		"name":        "ykc.kb_search",
		"description": "查詢 YKC 內嵌、唯讀 Rust 知識庫，回傳可直接提供給代理的最小上下文（錯誤碼、規則、官方文檔閉包）。結果附資料集版本與 rustc 版本，可審計、可重放。",
		"inputSchema": map[string]any{
			"type":     "object",
			"required": []string{"query"},
			"properties": map[string]any{
				"query":  map[string]any{"type": "string", "description": "錯誤訊息、Rust 程式片段或問題（最多 8 KiB）"},
				"k":      map[string]any{"type": "integer", "description": "初始命中數，1–20（預設 8）"},
				"expand": map[string]any{"type": "integer", "description": "知識圖展開深度，0–4（預設 2）"},
				"budget": map[string]any{"type": "integer", "description": "上下文預算 bytes，512–32768（預設 12000）"},
			},
		},
	},
	{
		"name":        "ykc.kb_explain",
		"description": "精確取得一個 Rust 錯誤碼、YKC 規則 ID 或官方文檔章節的知識閉包，含內容定址原子 ID、來源與 rustc 版本。這是解釋工具，不取代 rustc 判定。",
		"inputSchema": map[string]any{
			"type":     "object",
			"required": []string{"code"},
			"properties": map[string]any{
				"code":   map[string]any{"type": "string", "description": "E0382、BRW-01 或章節 ID"},
				"expand": map[string]any{"type": "integer", "description": "知識圖展開深度，0–4（預設 2）"},
				"budget": map[string]any{"type": "integer", "description": "上下文預算 bytes，512–32768（預設 12000）"},
			},
		},
	},
}

var (
	mcpKBOnce  sync.Once
	mcpKBStore *kb.Store
	mcpKBErr   error
)

// openMCPKnowledge 讓長駐 stdio MCP 進程只初始化一次唯讀 Store；Store 建成後
// 僅查詢（快取自身有鎖），可安全供每個 tools/call 重用。
func openMCPKnowledge() (*kb.Store, error) {
	mcpKBOnce.Do(func() {
		mcpKBStore, mcpKBErr = kb.Open()
	})
	return mcpKBStore, mcpKBErr
}

const (
	maxMCPKBQueryBytes = 8 << 10
	minMCPKBBudget     = 512
	maxMCPKBBudget     = 32 << 10
)

func mcpStringArg(args map[string]any, key string) string {
	v, _ := args[key].(string)
	return strings.TrimSpace(v)
}

// mcpBoundedIntArg 接受 encoding/json 解出的 float64，以及直接單元測試常用的
// int/json.Number；拒絕小數、字串和超出界限的值，避免代理用巨型檢索參數耗盡上下文。
func mcpBoundedIntArg(args map[string]any, key string, def, min, max int) (int, error) {
	v, ok := args[key]
	if !ok || v == nil {
		return def, nil
	}
	var n int64
	switch x := v.(type) {
	case int:
		n = int64(x)
	case int64:
		n = x
	case float64:
		if math.IsNaN(x) || math.IsInf(x, 0) || math.Trunc(x) != x {
			return 0, fmt.Errorf("%s 必須是整數", key)
		}
		// 先在 float 範圍內檢查，再轉 int64，避免惡意極大 JSON number 的
		// 實作相依溢位轉換。
		if x < float64(min) || x > float64(max) {
			return 0, fmt.Errorf("%s 必須介於 %d 與 %d", key, min, max)
		}
		return int(x), nil
	case json.Number:
		parsed, err := x.Int64()
		if err != nil {
			return 0, fmt.Errorf("%s 必須是整數", key)
		}
		n = parsed
	default:
		return 0, fmt.Errorf("%s 必須是整數", key)
	}
	if n < int64(min) || n > int64(max) {
		return 0, fmt.Errorf("%s 必須介於 %d 與 %d", key, min, max)
	}
	return int(n), nil
}

func mcpKBHeader(st *kb.Store) string {
	rustc := st.RustcVersion()
	if rustc == "" {
		rustc = "legacy/unknown"
	}
	return fmt.Sprintf("> KB dataset=%s · rustc=%s · source=%s\n\n", st.Version(), rustc, st.Source())
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

	case "ykc.kb_search":
		query := mcpStringArg(args, "query")
		if query == "" {
			return toolResult("需要 query 參數", true)
		}
		if len(query) > maxMCPKBQueryBytes {
			return toolResult(fmt.Sprintf("query 超過 %d bytes 上限", maxMCPKBQueryBytes), true)
		}
		k, err := mcpBoundedIntArg(args, "k", 8, 1, 20)
		if err != nil {
			return toolResult(err.Error(), true)
		}
		expand, err := mcpBoundedIntArg(args, "expand", 2, 0, 4)
		if err != nil {
			return toolResult(err.Error(), true)
		}
		budget, err := mcpBoundedIntArg(args, "budget", 12000, minMCPKBBudget, maxMCPKBBudget)
		if err != nil {
			return toolResult(err.Error(), true)
		}
		st, err := openMCPKnowledge()
		if err != nil {
			return toolResult("知識庫不可用: "+err.Error(), true)
		}
		bundle := st.Retrieve(query, kb.SearchOpts{K: k, ExpandDepth: expand, BudgetBytes: budget})
		return toolResult(mcpKBHeader(st)+kb.RenderMarkdown(bundle), false)

	case "ykc.kb_explain":
		code := mcpStringArg(args, "code")
		if code == "" {
			return toolResult("需要 code 參數", true)
		}
		if len(code) > 256 {
			return toolResult("code 超過 256 bytes 上限", true)
		}
		expand, err := mcpBoundedIntArg(args, "expand", 2, 0, 4)
		if err != nil {
			return toolResult(err.Error(), true)
		}
		budget, err := mcpBoundedIntArg(args, "budget", 12000, minMCPKBBudget, maxMCPKBBudget)
		if err != nil {
			return toolResult(err.Error(), true)
		}
		st, err := openMCPKnowledge()
		if err != nil {
			return toolResult("知識庫不可用: "+err.Error(), true)
		}
		atom, ok := st.ByCode(code)
		if !ok {
			return toolResult("找不到知識原子 "+code, true)
		}
		bundle := st.Retrieve(atom.Code, kb.SearchOpts{K: 1, ExpandDepth: expand, BudgetBytes: budget})
		return toolResult(mcpKBHeader(st)+fmt.Sprintf("> 精確根原子=%s\n\n", atom.ID)+kb.RenderMarkdown(bundle), false)

	case "ykc.borrow_rules":
		code, _ := args["code"].(string)
		if code == "" {
			return toolResult(borrow.RenderCard(borrow.RuleCard), false)
		}
		entries := borrow.ByRustcCode(code)
		if len(entries) == 0 {
			if !borrow.IsBorrowCode(code) {
				return toolResult(fmt.Sprintf("%s 不屬 borrow/生命週期類錯誤（本卡只覆蓋借用幾何）", code), false)
			}
			return toolResult(fmt.Sprintf("%s 屬 borrow 類但無專屬條目; 全卡如下:\n\n%s",
				code, borrow.RenderCard(borrow.RuleCard)), false)
		}
		return toolResult(borrow.RenderCard(entries), false)

	case "ykc.borrow_explain":
		src, _ := args["cl_source"].(string)
		if src == "" {
			return toolResult("需要 cl_source 參數（.cl 最小樣例源碼）", true)
		}
		analyzer := &borrow.Analyzer{}
		if avail, why := analyzer.Available(); !avail {
			return toolResult("L5 引擎不可用: "+why+"（規則卡仍可用: ykc.borrow_rules）", true)
		}
		r, err := analyzer.AnalyzeSource(context.Background(), dir, src)
		if err != nil {
			return toolResult("L5 分析失敗: "+err.Error(), true)
		}
		topo := borrow.BuildTopology(r)
		var sb strings.Builder
		sb.WriteString(topo.RenderText())
		fmt.Fprintf(&sb, "\nverdict(僅指此 .cl 模型): %s", r.Verdict)
		fmt.Fprintf(&sb, "\n紅邊(違法重疊): %d — 修復收斂判據: 紅邊清零", topo.RedEdges())
		for _, e := range r.Errors {
			fmt.Fprintf(&sb, "\n  [%s] %s 「%s」— %s", e.Code, e.Stmt, e.StmtText, e.Message)
		}
		return toolResult(sb.String(), false)

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
