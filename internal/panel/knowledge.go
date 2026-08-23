package panel

import (
	"net/http"
	"strings"
	"sync"

	"ykc/internal/kb"
)

var (
	knowledgeOnce    sync.Once
	knowledgeStore   *kb.Store
	knowledgeInitErr error
)

// KnowledgeHandler 回傳 Trust Console 的唯讀知識面。它把 `/api/know/*` 映射到
// internal/kb 的 `/api/kb/*` handler；所有端點維持 GET、無 token、無寫入，與
// `/api/state` 同屬觀察面。Store 只初始化一次，讓 panel 與 serve 共用同一份索引。
func KnowledgeHandler() http.Handler {
	knowledgeOnce.Do(func() {
		knowledgeStore, knowledgeInitErr = kb.Open()
	})
	if knowledgeInitErr != nil {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				http.Error(w, "GET only", http.StatusMethodNotAllowed)
				return
			}
			http.Error(w, "kb unavailable: "+knowledgeInitErr.Error(), http.StatusServiceUnavailable)
		})
	}
	h := kb.Handler(knowledgeStore)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "GET only", http.StatusMethodNotAllowed)
			return
		}
		r2 := r.Clone(r.Context())
		suffix := strings.TrimPrefix(r.URL.Path, "/api/know")
		if suffix == "" {
			suffix = "/"
		}
		r2.URL.Path = "/api/kb" + suffix
		h.ServeHTTP(w, r2)
	})
}
