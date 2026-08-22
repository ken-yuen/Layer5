package kb

import (
	"encoding/json"
	"net/http"
	"sort"
	"strconv"
	"strings"
)

// Handler 回傳知識庫的「唯讀 HTTP API」（供 AI agent / 外部系統拉取）。
// 全部端點皆 GET 且唯讀——與 YKC 面板「觀察（唯讀）」的安全邊界一致：
// 不產生、不改寫任何資料，可安全供代理無 token 存取。
//
//	GET /healthz              → ok
//	GET /api/kb/state         → 資料版本、原子統計、緩存統計
//	GET /api/kb/search?q=&k=&expand=&budget=&format=json|md
//	GET /api/kb/code/{CODE}   → 錯誤碼卡（E0382 等）
//	GET /api/kb/rule/{ID}     → 規則（OWN-01 等）
//	GET /api/kb/rules?domain= → 規則列表（可依領域篩選）
//	GET /api/kb/graph?code=&depth= → 依賴項圖展開（代理用）
//	GET /api/kb/book          → 官方教學文檔目錄
//	GET /api/kb/book/{id}     → 章節
//	GET /api/kb/codes         → 全部錯誤碼（含一行標題）
func Handler(s *Store) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("GET /api/kb/healthz", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})

	mux.HandleFunc("GET /api/kb/state", func(w http.ResponseWriter, r *http.Request) {
		kinds := map[string]int{}
		for _, a := range s.atoms {
			kinds[string(a.Kind)]++
		}
		cs := s.CacheStats()
		writeJSON(w, map[string]any{
			"version":    s.Version(),
			"source":     s.Source(),
			"atoms":      s.Count(),
			"kinds":      kinds,
			"domains":    s.Domains(),
			"cache":      cs,
			"cycle_sccs": len(s.Cycles()),
			"read_only":  true,
		})
	})

	mux.HandleFunc("GET /api/kb/search", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("q")
		if q == "" {
			http.Error(w, "需要 q 參數", http.StatusBadRequest)
			return
		}
		opts := SearchOpts{
			K:           atoiDefault(r.URL.Query().Get("k"), 8),
			ExpandDepth: atoiDefault(r.URL.Query().Get("expand"), 2),
			BudgetBytes: atoiDefault(r.URL.Query().Get("budget"), 12000),
		}
		b := s.Retrieve(q, opts)
		if r.URL.Query().Get("format") == "md" {
			w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
			_, _ = w.Write([]byte(RenderMarkdown(b)))
			return
		}
		writeJSON(w, b)
	})

	mux.HandleFunc("GET /api/kb/code/{code}", func(w http.ResponseWriter, r *http.Request) {
		a, ok := s.ByCode(r.PathValue("code"))
		if !ok || a.Kind != KindError {
			http.NotFound(w, r)
			return
		}
		writeJSON(w, a)
	})

	mux.HandleFunc("GET /api/kb/rule/{id}", func(w http.ResponseWriter, r *http.Request) {
		a, ok := s.ByCode(r.PathValue("id"))
		if !ok || a.Kind != KindRule {
			http.NotFound(w, r)
			return
		}
		writeJSON(w, a)
	})

	mux.HandleFunc("GET /api/kb/rules", func(w http.ResponseWriter, r *http.Request) {
		domain := r.URL.Query().Get("domain")
		var rules []*Atom
		if domain == "" {
			rules = s.AtomsByKind(KindRule)
		} else {
			rules = s.RulesByDomain(domain)
		}
		writeJSON(w, map[string]any{"domain": domain, "rules": rules})
	})

	mux.HandleFunc("GET /api/kb/graph", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		if code == "" {
			code = r.URL.Query().Get("id")
		}
		var root []string
		if a, ok := s.ByCode(code); ok {
			root = []string{a.ID}
		} else if a, ok := s.ByID(code); ok {
			root = []string{a.ID}
		} else {
			http.Error(w, "需要 code 或 id 參數（且須存在）", http.StatusBadRequest)
			return
		}
		depth := atoiDefault(r.URL.Query().Get("depth"), 2)
		atoms := s.ExpandContext(root, depth)
		type edge struct {
			From string `json:"from"`
			To   string `json:"to"`
		}
		var edges []edge
		idset := map[string]bool{}
		for _, a := range atoms {
			idset[a.ID] = true
		}
		for _, a := range atoms {
			for _, dep := range s.Deps(a.ID) {
				if idset[dep.ID] {
					edges = append(edges, edge{a.ID, dep.ID})
				}
			}
		}
		writeJSON(w, map[string]any{"root": root[0], "depth": depth, "nodes": atoms, "edges": edges})
	})

	mux.HandleFunc("GET /api/kb/book", func(w http.ResponseWriter, r *http.Request) {
		parts := s.AtomsByKind(KindPart)
		chapters := s.AtomsByKind(KindBook)
		writeJSON(w, map[string]any{"parts": parts, "chapters": chapters})
	})

	mux.HandleFunc("GET /api/kb/book/{id}", func(w http.ResponseWriter, r *http.Request) {
		a, ok := s.ByCode(r.PathValue("id"))
		if !ok || (a.Kind != KindBook && a.Kind != KindPart) {
			http.NotFound(w, r)
			return
		}
		writeJSON(w, a)
	})

	mux.HandleFunc("GET /api/kb/codes", func(w http.ResponseWriter, r *http.Request) {
		errs := s.AtomsByKind(KindError)
		// 依數字序。
		sort.Slice(errs, func(i, j int) bool {
			return numOf(errs[i].Code) < numOf(errs[j].Code)
		})
		type codeInfo struct {
			Code  string `json:"code"`
			Title string `json:"title"`
		}
		out := make([]codeInfo, 0, len(errs))
		for _, a := range errs {
			out = append(out, codeInfo{a.Code, a.Title})
		}
		writeJSON(w, map[string]any{"count": len(out), "codes": out})
	})

	return mux
}

func numOf(code string) int {
	n, _ := strconv.Atoi(strings.TrimPrefix(strings.ToUpper(code), "E"))
	return n
}

func atoiDefault(s string, def int) int {
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return n
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}
