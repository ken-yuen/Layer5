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
#   make health   — 全專案健檢（vet + build + 回歸）
#   make image   — 建置 OCI 鏡像（Podman 優先，回退 Docker）
#   make up      — 本機容器一鍵運行
.PHONY: setup verify-all build binaries smoke judge lsp guard guard-verify guard-score guard-mcp panel health image up clean

# 工具鏈位置：預設 $HOME/.ykc（零 sudo）；可用環境變數覆寫（如 YKC_HOME=/opt/ykc）
# 注意：RUSTUP_HOME / CARGO_HOME 用「?=」尊重環境已設值——
#   本地：launch.sh / dev-setup.sh 會設好；CI：dtolnay/rust-toolchain 裝在 $HOME/.rustup + $HOME/.cargo，
#   若 Makefile 強行覆寫會令 CI 的 rustup 找不到 toolchain。
YKC_HOME ?= $(HOME)/.ykc
export RUSTUP_HOME ?= $(YKC_HOME)/rustup
export CARGO_HOME ?= $(YKC_HOME)/cargo
export PATH := $(YKC_HOME)/bin:$(YKC_HOME)/go/bin:$(YKC_HOME)/cargo/bin:$(PATH)

setup:
	bash dev-setup.sh

# 一鍵跑全部功能實測（朋友下載後驗證：全部應通過；冒號後為「預期行為」）
verify-all: build
	@go build -o bin/ykc-judge ./cmd/ykc-judge
	@go build -o bin/ykc-guard ./cmd/ykc-guard
	@go build -o bin/ykc-lsp ./cmd/ykc-lsp
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
	@echo " ⑧ MCP server → 預期三工具可用"
	@python3 ./test-mcp-client.py ./bin/ykc-guard ./demo-rust-cli | tail -1
	@echo " ⑨ LSP 診斷 → 預期 initialize 成功"
	@./bin/ykc-lsp ./demo-semantic-cli/src/main.rs rust-analyzer 2>/dev/null | head -1
	@echo "=============================================================="
	@echo "✅ 全功能實測完成（②⑦ 的 FAIL/TAKEOVER 為反欺騙的預期行為）"

build:
	mkdir -p bin
	CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o bin/ykc ./cmd/ykc-smoke

# 建全部五個二進制（launch.sh 用；不執行任何動作）
binaries:
	mkdir -p bin
	CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o bin/ykc ./cmd/ykc-smoke
	go build -o bin/ykc-judge ./cmd/ykc-judge
	go build -o bin/ykc-guard ./cmd/ykc-guard
	go build -o bin/ykc-lsp ./cmd/ykc-lsp
	go build -o bin/ykc-panel ./cmd/ykc-panel

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

smoke: build
	./bin/ykc -dir ./demo-rust-cli -claims ./claims.json -key $${YKC_KEY:-ykc-dev-key}

health:
	@echo "== go vet =="; go vet ./...
	@echo "== go build =="; go build ./...
	@echo "== smoke =="; ./bin/ykc -dir ./demo-rust-cli -claims ./claims.json -key ykc-dev-key >/dev/null 2>&1 && echo "  smoke OK" || echo "  smoke FAIL"
	@echo "== judge gate =="; ./bin/ykc-judge -dir ./demo-broken-cli -gate >/dev/null 2>&1 && echo "  judge OK" || echo "  judge FAIL"
	@echo "== ledger verify =="; ./bin/ykc-judge -dir ./demo-broken-cli -verify >/dev/null 2>&1 && echo "  ledger OK" || echo "  ledger FAIL"

image:
	podman build -t ykc:latest -f deploy/Dockerfile . || docker build -t ykc:latest -f deploy/Dockerfile .

up:
	podman compose -f deploy/compose.yaml up || docker compose -f deploy/compose.yaml up

clean:
	rm -rf bin demo-rust-cli/target
