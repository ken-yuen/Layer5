#!/usr/bin/env bash
# YKC 啟動器（macOS 雙擊用：launch.command；Linux/WSL：launch.sh）
# 效果：裝環境 → 建二進制 → 起控制面板 → 自動開瀏覽器。
set -e
cd "$(dirname "$0")"

export YKC_HOME="${YKC_HOME:-$HOME/.ykc}"

# 1) 安裝環境（冪等：已有則秒過）
bash dev-setup.sh

# 2) 讓面板的子行程（judge/smoke 內呼叫 cargo/go）找得到工具鏈
export PATH="$YKC_HOME/bin:$YKC_HOME/go/bin:$YKC_HOME/cargo/bin:$PATH"
export RUSTUP_HOME="$YKC_HOME/rustup"
export CARGO_HOME="$YKC_HOME/cargo"

# 3) 建全部二進制
make binaries

# 4) 開瀏覽器（延後讓伺服器先起）
( sleep 1; open "http://127.0.0.1:8080" 2>/dev/null || xdg-open "http://127.0.0.1:8080" >/dev/null 2>&1 || true ) &

# 5) 起面板（本機綁 127.0.0.1，較安全）
echo "── YKC Trust Console 啟動中，瀏覽器將自動開啟 http://127.0.0.1:8080 ──"
exec ./bin/ykc-panel -root . -port 8080 -addr 127.0.0.1:8080
