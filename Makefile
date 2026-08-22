# YKC 一鍵命令（任何裝置）
#   make setup   — 一鍵還原開發環境（冪等、零 sudo、跨平台）
#   make verify-all — 一鍵跑全部功能實測（朋友下載後驗證用）
#   make build   — 編譯煙測引擎（零依賴靜態二進制）
#   make smoke   — 對 demo 專案跑煙測（原生，無需容器）
#   make judge   — L4 除錯閉環（demo-broken-cli）
#   make lsp     — LSP 客戶端測試
#   make guard   — 編譯動態護欄（反欺騙裁判）
#   make guard-verify   — T-14 確定性比對（誠實 vs 撒謊）
#   make guard-score    — T-15 信任棘輪（撒謊代理 → T0）
#   make guard-mcp      — T-17 MCP server 測試
#   make panel    — 啟動 YKC Trust Console（唯讀觀察台，port 8080）
#   make precompile — Rust cargo/rustc 預編譯檢查
#   make health   — 全專案健檢（gofmt + vet + staticcheck + build + 回歸）
#   make anchor-test — 獨立 head anchor（截斷／重簽／remote witness）回歸
#   make know-import / know-replay / know-diff — KB release 的建庫、可重放與審計差異
#   make deps / deps-test / deps-setup — T-19 L1 capability worker、fixtures、顯式鎖版工具
#   make structure / structure-test — T-20 純 Go Rust/Go grammar capability worker
#   make image   — 建置 OCI 鏡像（Podman 優先，回退 Docker）
#   make up      — 本機容器一鍵運行
.PHONY: setup verify-all build binaries smoke atom precompile judge lsp guard guard-verify guard-score guard-mcp panel serve health image up clean l5-test borrow-test anchor-test cap pack-deps pack-structure deps deps-test deps-setup structure structure-test structure-size thin-core-test capability-test know know-test know-build know-import know-replay know-diff fmt-check vet staticcheck lint

# 工具鏈位置：預設 $HOME/.ykc（零 sudo）；可用環境變數覆寫（如 YKC_HOME=/opt/ykc）
YKC_HOME ?= $(HOME)/.ykc

# 僅當 YKC_HOME 下「真的存在」rustup/cargo 目錄時才 export（本地 dev-setup 場景）。
# CI 的 rust 由 dtolnay 裝在默認 $HOME/.rustup（且不 export RUSTUP_HOME）；
# 若 Makefile 強行設 RUSTUP_HOME 會令 rustup 找不到 toolchain → cargo 立即失敗。
YKC_RUSTUP_HOME := $(shell test -d "$(YKC_HOME)/rustup" && echo "$(YKC_HOME)/rustup")
YKC_CARGO_HOME := $(shell test -d "$(YKC_HOME)/cargo" && echo "$(YKC_HOME)/cargo")
ifneq ($(YKC_RUSTUP_HOME),)
export RUSTUP_HOME := $(YKC_RUSTUP_HOME)
endif
ifneq ($(YKC_CARGO_HOME),)
export CARGO_HOME := $(YKC_CARGO_HOME)
endif
export PATH := $(YKC_HOME)/bin:$(YKC_HOME)/go/bin:$(YKC_HOME)/cargo/bin:$(PATH)

# ── 合入品質閘門（YKC_17 / R4）────────────────────────────────────
# staticcheck 是開發/CI 工具，不進入 YKC 的執行期依賴圖。`make setup` 會安裝
# 鎖定版本；若使用自行管理的 Go，這裡也會探測 $(go env GOPATH)/bin。
fmt-check:
	@bad="$$(gofmt -l cmd internal core)"; test -z "$$bad" || { echo "以下檔案未 gofmt："; echo "$$bad"; exit 1; }

vet:
	@go vet ./...

staticcheck:
	@tool="$$(command -v staticcheck 2>/dev/null || true)"; \
	if [ -z "$$tool" ]; then candidate="$$(go env GOPATH 2>/dev/null)/bin/staticcheck"; test -x "$$candidate" && tool="$$candidate"; fi; \
	test -n "$$tool" || { echo "找不到 staticcheck；請先執行 make setup，或執行：GOBIN=\"$(YKC_HOME)/bin\" go install honnef.co/go/tools/cmd/staticcheck@v0.8.1"; exit 1; }; \
	"$$tool" -checks=all ./...

# T-21a 解耦守衛：internal/toolchain 之外禁止直接 exec cargo/rustc
#（internal/precompile 走沙盒 runStage、internal/sandbox 為執行器，屬既有豁免路徑）。
toolchain-guard:
	@bad="$$(grep -rn 'rustutil\.Run(.*"cargo"\|rustutil\.Run(.*"rustc"\|exec\.Command(.*"cargo"\|exec\.Command(.*"rustc"' --include='*.go' cmd internal core 2>/dev/null | grep -v '^internal/toolchain/' || true)"; \
	test -z "$$bad" || { echo "❌ toolchain-guard：以下位置繞過 core.RustToolchain port 直呼 cargo/rustc："; echo "$$bad"; exit 1; }
	@echo "✅ toolchain-guard：cargo/rustc 全部經 port 出入"

lint: fmt-check vet staticcheck toolchain-guard

setup:
	bash dev-setup.sh

# 受限環境備援：go.dev 不可達時，自 GitHub 源碼六級 bootstrap 鏈自建 Go（見 bootstrap-go.sh）
bootstrap-go:
	bash bootstrap-go.sh

# 一鍵跑全部功能實測（朋友下載後驗證：全部應通過；冒號後為「預期行為」）
verify-all: lint build
	@go build -o bin/ykc-judge ./cmd/ykc-judge
	@go build -o bin/ykc-guard ./cmd/ykc-guard
	@go build -o bin/ykc-lsp ./cmd/ykc-lsp
	@go build -o bin/ykc-precompile ./cmd/ykc-precompile
	@go build -o bin/ykc-serve ./cmd/ykc-serve
	@echo "=============================================================="
	@echo " ① 煙測引擎（健康專案，無謊報）→ 預期 PASS"
	@./bin/ykc -dir ./demo-rust-cli -key ykc-dev-key | grep 整體判定
	@echo " ② 煙測反欺騙（2 條謊報聲明）→ 預期 FAIL（正確揪出謊報）"
	@./bin/ykc -dir ./demo-rust-cli -claims ./claims.json -key ykc-dev-key | grep 整體判定
	@echo " ③ L4 除錯閘門 → 預期通過"
	@./bin/ykc-judge -dir ./demo-broken-cli -gate
	@echo " ④ L4 除錯閉環（產生簽名收據+帳本）→ 預期 0 錯誤"
	@./bin/ykc-judge -dir ./demo-broken-cli -key ykc-dev-key | grep 整體判定
	@echo " ⑤ 帳本完整性驗證 → 預期完整"
	@./bin/ykc-judge -dir ./demo-broken-cli -verify
	@echo " ⑥ 護欄-誠實代理 → 預期 🟢 PASS"
	@./bin/ykc-guard score -dir ./demo-rust-cli -claims ./demo-agent-honest.json | grep 整體判定
	@echo " ⑦ 護欄-撒謊代理 → 預期 🔴 TAKEOVER"
	@./bin/ykc-guard score -dir ./demo-semantic-cli -claims ./demo-agent-lying.json | grep 整體判定
	@echo " ⑧ MCP server → 預期 7 工具可用（含 KB ×2）"
	@python3 ./test-mcp-client.py ./bin/ykc-guard ./demo-rust-cli | tail -1
	@echo " ⑨ LSP 診斷 → 預期 initialize 成功"
	@./bin/ykc-lsp ./demo-semantic-cli/src/main.rs rust-analyzer 2>/dev/null | head -1
	@echo " ⑩ rustc/cargo 預編譯 → 預期 PASS"
	@./bin/ykc-precompile -project ./demo-rust-cli -sandbox native -allow-native -json=false | head -1
	@echo " ⑪ ykc serve 常駐進程 → 預期 healthz=ok；無證據聲明被 datalog 護欄駁回"
	@go build -o bin/ykc-serve ./cmd/ykc-serve
	@./bin/ykc-serve -root . -port 18099 >/tmp/ykc-serve-verify.log 2>&1 & SERVER_PID=$$!; 	  for i in 1 2 3 4 5 6 7 8 9 10; do curl -sf http://127.0.0.1:18099/healthz >/dev/null 2>&1 && break; sleep 0.5; done; 	  test "$$(curl -sf http://127.0.0.1:18099/healthz)" = "ok" || { echo "serve healthz FAIL"; cat /tmp/ykc-serve-verify.log; kill $$SERVER_PID; exit 1; }; 	  echo "  healthz OK"; 	  CLAIMS_RESP=$$(curl -s -X POST http://127.0.0.1:18099/api/claims -d '{"project":"./demo-broken-cli","kind":"tests_passed"}'); 	  echo "$$CLAIMS_RESP" | grep -q fake_test_claim && echo "  datalog 護欄駁回 OK" || { echo "claims guardrail FAIL: $$CLAIMS_RESP"; kill $$SERVER_PID; exit 1; }; 	  curl -sf http://127.0.0.1:18099/api/watch | grep -q '"backend"' && echo "  /api/watch OK" || { echo "api/watch FAIL"; kill $$SERVER_PID; exit 1; }; 	  kill $$SERVER_PID; sleep 0.5; 	  (kill -0 $$SERVER_PID 2>/dev/null && { echo "serve 未優雅退出"; exit 1; }) || echo "  優雅退出 OK"
	@echo "  ⑫ 獨立 head anchor → 預期截斷／重簽／遠端 witness 防線全綠"
	@$(MAKE) --no-print-directory anchor-test >/dev/null && echo "  head anchor OK"
	@echo "  ⑬ T-19/T-20 capability admission → metadata/audit fixture + pure-Go Rust/Go grammar size boundary"
	@$(MAKE) --no-print-directory capability-test >/dev/null && echo "  L1/L2 capability admission OK"
	@echo "=============================================================="
	@echo "✅ 全功能實測完成（②⑦ 的 FAIL/TAKEOVER 為反欺騙的預期行為）"

build:
	mkdir -p bin
	CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o bin/ykc ./cmd/ykc-smoke

# 建全部二進制（launch.sh 用；不執行任何動作）
# 注：ykc-cap/ykc-deps/ykc-structure（YKC_21 能力包）代碼尚未落庫，
#     落庫後再恢復對應 build 行——binaries 不得引用不存在的套件（T-21 修正）。
binaries:
	mkdir -p bin
	CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o bin/ykc ./cmd/ykc-smoke
	go build -o bin/ykc-judge ./cmd/ykc-judge
	go build -o bin/ykc-guard ./cmd/ykc-guard
	go build -o bin/ykc-lsp ./cmd/ykc-lsp
	go build -o bin/ykc-panel ./cmd/ykc-panel
	go build -o bin/ykc-atom ./cmd/ykc-atom
	go build -o bin/ykc-precompile ./cmd/ykc-precompile
	go build -o bin/ykc-serve ./cmd/ykc-serve
	go build -o bin/ykc-know ./cmd/ykc-know
	go build -o bin/ykc-doctor ./cmd/ykc-doctor
	go build -o bin/ykc-rustd ./cmd/ykc-rustd

atom:
	mkdir -p bin
	go build -o bin/ykc-atom ./cmd/ykc-atom
	@echo "ykc-atom built: bin/ykc-atom"

precompile:
	mkdir -p bin
	go build -o bin/ykc-precompile ./cmd/ykc-precompile
	./bin/ykc-precompile -project ./demo-rust-cli -sandbox native -allow-native -json=false

judge:
	go build -o bin/ykc-judge ./cmd/ykc-judge
	./bin/ykc-judge -dir ./demo-broken-cli -key $${YKC_KEY:-ykc-dev-key}

lsp:
	go build -o bin/ykc-lsp ./cmd/ykc-lsp
	./bin/ykc-lsp ./demo-semantic-cli/src/main.rs rust-analyzer

guard:
	go build -o bin/ykc-guard ./cmd/ykc-guard

guard-verify: guard
	@echo "── 誠實代理 ──"; ./bin/ykc-guard verify -dir ./demo-rust-cli -claims ./demo-agent-honest.json
	@echo "── 撒謊代理 ──"; ./bin/ykc-guard verify -dir ./demo-semantic-cli -claims ./demo-agent-lying.json

guard-score: guard
	./bin/ykc-guard score -dir ./demo-semantic-cli -claims ./demo-agent-lying.json

guard-mcp: guard
	python3 ./test-mcp-client.py ./bin/ykc-guard ./demo-rust-cli

panel: binaries
	./bin/ykc-panel -root . -port 8080

# 常駐進程（YKC_14）：監看+聲明評估+面板合一（atom+judge+guard+panel 的運行時面）
serve: binaries
	./bin/ykc-serve -root . -port 8080

smoke: build
	./bin/ykc -dir ./demo-rust-cli -claims ./claims.json -key $${YKC_KEY:-ykc-dev-key}

# L5 引擎回歸（vendored ChordLaw 上游 19 項測試）
l5-test:
	python3 l5/chordlaw/test_chordlaw.py

# L5 Go 接線層測試（拓撲/規則卡/golden；python3 缺席時 E2E 自動 Skip）
borrow-test:
	go test ./internal/borrow/...

# ── T-21 工具鏈解耦（YKC_22）────────────────────────────────────────
# 零工具鏈測試（core-lane）：replay/unavailable adapter + LSP 假 server，全綠不需 Rust。
toolchain-test:
	go test ./internal/toolchain/... ./internal/lsp/... ./internal/serve/ -run 'Test' -count=1

# 契約測試（toolchain-lane）：真實鎖版 cargo 驗證診斷 JSON 契約未漂移；
# YKC_REQUIRE_TOOLCHAIN=1 時「無 cargo」由 skip 轉為 fail（CI 不允許靜默降級）。
contract-test:
	YKC_REQUIRE_TOOLCHAIN=1 go test ./internal/toolchain/ -run 'TestContract' -v -count=1

# T-24：獨立 head anchor 的截斷／重簽／遠端 witness 回歸。
anchor-test:
	go test ./internal/ledger/... -run 'TestHeadAnchor|TestOptionalRemoteWitness|TestRequiredRemoteWitness' -count=1

# ── Capability pack core（manifest verification + JSONL worker composition）─
cap:
	mkdir -p bin
	go build -o bin/ykc-cap ./cmd/ykc-cap
	@echo "ykc-cap built: bin/ykc-cap"

# ── T-19 L1 capability pack（Cargo dependency evidence）────────────
# 工具安裝是顯式、鎖版行為；scan 本身絕不隱式下載 audit DB 或 Cargo tool。
CARGO_AUDIT_VERSION ?= 0.22.2
CARGO_DENY_VERSION ?= 0.20.2
STRUCTURE_TAGS := grammar_subset grammar_subset_rust grammar_subset_go

deps:
	mkdir -p bin
	go build -o bin/ykc-deps ./cmd/ykc-deps
	@echo "ykc-deps built: bin/ykc-deps"

deps-test:
	go test ./internal/capability/... ./internal/deps/... ./cmd/ykc-deps/...

# 需人類明確執行：安裝與 rustc 1.98 相容性應先在 CI/OCI admission 驗證。
deps-setup:
	cargo install cargo-audit --version "$(CARGO_AUDIT_VERSION)" --locked
	cargo install cargo-deny --version "$(CARGO_DENY_VERSION)" --locked
	@cargo audit --version
	@cargo deny --version

# ── T-20 L2 capability pack（pure-Go Rust/Go structure evidence）────
# grammar_subset tags 只嵌 Rust + Go blob，避免把 206 grammar 塞入 worker binary。
structure:
	mkdir -p bin
	go build -tags="$(STRUCTURE_TAGS)" -o bin/ykc-structure ./cmd/ykc-structure
	@echo "ykc-structure built (Rust+Go grammar subset): bin/ykc-structure"

structure-test:
	go test -tags="$(STRUCTURE_TAGS)" ./internal/structure/... ./cmd/ykc-structure/...

# Admission guard: syntax grammars belong to the optional worker, never ykc T0 core.
thin-core-test:
	@! go list -deps ./cmd/ykc-smoke | grep -qx 'github.com/odvcencio/gotreesitter' || { echo "T0 core unexpectedly links gotreesitter"; exit 1; }
	@echo "thin core dependency boundary OK"

# Rust+Go subset currently budgets <= 25 MiB; all grammar blobs are deliberately excluded.
structure-size: structure
	@bytes=$$(wc -c < bin/ykc-structure); test "$$bytes" -le 26214400 || { echo "ykc-structure exceeds 25 MiB: $$bytes"; exit 1; }; echo "ykc-structure bytes=$$bytes (<=25 MiB)"

# Build content-addressed local development packs. They are optional outputs;
# ykc-core itself remains the small T0 binary and only runs packs after manifest verification.
pack-deps: cap deps
	@audit="$$(command -v cargo-audit || true)"; deny="$$(command -v cargo-deny || true)"; \
	  test -n "$$audit" && test -n "$$deny" || { echo "需要 pinned cargo tools；先執行 make deps-setup"; exit 2; }; \
	  root="packs/deps/0.1.0/$$(go env GOOS)-$$(go env GOARCH)"; \
	  rm -rf "$$root"; mkdir -p "$$root/bin" "$$root/tools"; cp bin/ykc-deps "$$root/bin/ykc-deps"; cp "$$audit" "$$root/tools/cargo-audit"; cp "$$deny" "$$root/tools/cargo-deny"; \
	  bin/ykc-cap manifest -root "$$root" -id deps -version 0.1.0 -entry bin/ykc-deps -artifacts tools/cargo-audit,tools/cargo-deny -tools cargo-audit@$(CARGO_AUDIT_VERSION),cargo-deny@$(CARGO_DENY_VERSION) -capabilities dependency.scan; \
	  echo "deps pack: $$root"

pack-structure: cap structure
	@root="packs/structure-rustgo/0.1.0/$$(go env GOOS)-$$(go env GOARCH)"; \
	  rm -rf "$$root"; mkdir -p "$$root/bin"; cp bin/ykc-structure "$$root/bin/ykc-structure"; \
	  bin/ykc-cap manifest -root "$$root" -id structure-rustgo -version 0.1.0 -entry bin/ykc-structure -capabilities structure.scan; \
	  echo "structure pack: $$root"

capability-test: deps-test structure-test thin-core-test structure-size

# ── 知識庫 + 代理上下文引擎（YKC_15）──────────────────────────────
# 嵌入式唯讀知識庫：518 條 rustc 錯誤碼（含錯誤範例+正解）、54 條規則抽象、
# 官方教學文檔（19 部 / 91 章）；精準檢索 + 依賴項圖 + 上下文緩存 + 預算截斷。
know-test:
	go test ./internal/kb/...

know:
	mkdir -p bin
	go build -o bin/ykc-know ./cmd/ykc-know
	./bin/ykc-know stats

# 建單一唯讀 blob 資料庫（開檔 sha256 校驗防竄改；含內嵌資料對應 rustc 版本）
know-build:
	mkdir -p bin
	go build -o bin/ykc-know ./cmd/ykc-know
	./bin/ykc-know build -o bin/kb.ykc

# 從目前 rustc 的對應官方 error index 重建版本鎖定 blob（顯式網路建庫；不進預設 verify）。
know-import:
	mkdir -p bin
	go build -o bin/ykc-know ./cmd/ykc-know
	./bin/ykc-know import "$$($${RUSTC:-rustc} --version)" -o bin/kb.ykc

# 依 release manifest 重抓官方來源並驗證 URL/ETag/SHA/原子/blob 都一致。
# 用法：make know-replay MANIFEST=bin/kb.ykc.manifest.json OUT=bin/kb-replay.ykc
know-replay:
	@test -n "$(MANIFEST)" || { echo "需要 MANIFEST=<kb.manifest.json>"; exit 2; }
	@test -n "$(OUT)" || { echo "需要 OUT=<replayed.ykc>"; exit 2; }
	mkdir -p bin
	go build -o bin/ykc-know ./cmd/ykc-know
	./bin/ykc-know replay "$(MANIFEST)" -o "$(OUT)"

# 比較兩份已校驗 KB blob 的 metadata、內容原子與 knowledge graph refs。
# 用法：make know-diff BASE=bin/old.ykc TARGET=bin/new.ykc
know-diff:
	@test -n "$(BASE)" || { echo "需要 BASE=<old.ykc>"; exit 2; }
	@test -n "$(TARGET)" || { echo "需要 TARGET=<new.ykc>"; exit 2; }
	mkdir -p bin
	go build -o bin/ykc-know ./cmd/ykc-know
	./bin/ykc-know diff "$(BASE)" "$(TARGET)"

# 唯讀 HTTP API（供 AI agent 拉取）
know-serve:
	mkdir -p bin
	go build -o bin/ykc-know ./cmd/ykc-know
	./bin/ykc-know serve -addr 127.0.0.1 -port 8090

health: lint binaries
	@echo "== go build =="; go build ./...
	@echo "== smoke =="; ./bin/ykc -dir ./demo-rust-cli -claims ./claims.json -key ykc-dev-key >/dev/null 2>&1 && echo "  smoke OK" || echo "  smoke FAIL"
	@echo "== judge gate =="; ./bin/ykc-judge -dir ./demo-broken-cli -gate >/dev/null 2>&1 && echo "  judge OK" || echo "  judge FAIL"
	@echo "== ledger verify =="; ./bin/ykc-judge -dir ./demo-broken-cli -verify >/dev/null 2>&1 && echo "  ledger OK" || echo "  ledger FAIL"

image:
	podman build -t ykc:latest -f deploy/Dockerfile . || docker build -t ykc:latest -f deploy/Dockerfile .

up:
	podman compose -f deploy/compose.yaml up || docker compose -f deploy/compose.yaml up

clean:
	rm -rf bin demo-rust-cli/target demo-rust-cli/.ykc/precompile demo-semantic-cli/.ykc/precompile demo-broken-cli/.ykc/precompile
