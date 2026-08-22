#!/usr/bin/env bash
# YKC 環境一鍵還原（冪等、零 sudo、跨平台：Linux / macOS / Windows-WSL）
#
# 用法：
#   bash dev-setup.sh            # 工具鏈裝到 $HOME/.ykc（預設，無需 sudo）
#   YKC_HOME=/opt/ykc bash dev-setup.sh   # 自訂位置
#
# 行為：
#   - 偵測系統已裝的 go / cargo，有則直接沿用，無則下載到 $YKC_HOME
#   - rust-analyzer 元件（供 LSP 客戶端測試）自動補裝
#   - 冪等：可重複執行，不會重複下載
set -e

GO_VER="1.27.0"
YKC_HOME="${YKC_HOME:-$HOME/.ykc}"
mkdir -p "$YKC_HOME"

# ---------- 平台偵測 ----------
OS="$(uname -s)"
case "$OS" in
  Linux)  GO_OS="linux" ;;
  Darwin) GO_OS="darwin" ;;
  *)      echo "❌ 不支援的系統：$OS（請在 WSL/Linux/macOS 執行）" >&2; exit 1 ;;
esac
case "$(uname -m)" in
  x86_64|amd64) GO_ARCH="amd64" ;;
  arm64|aarch64) GO_ARCH="arm64" ;;
  *) echo "❌ 不支援的架構：$(uname -m)" >&2; exit 1 ;;
esac

echo "── 平台：$GO_OS/$GO_ARCH ｜ 工具鏈位置：$YKC_HOME ──"

# ---------- Go ----------
# 優先沿用系統 Go；否則下載官方二進制到 $YKC_HOME/go（零 sudo）
if command -v go >/dev/null 2>&1; then
  echo "✅ 偵測到系統 Go：$(go version)"
else
  if [ -x "$YKC_HOME/go/bin/go" ]; then
    echo "✅ 偵測到 $YKC_HOME/go：$("$YKC_HOME/go/bin/go" version)"
  else
    echo "── 下載 Go $GO_VER ($GO_OS-$GO_ARCH) → $YKC_HOME/go ──"
    curl -sL -o "$YKC_HOME/go.tgz" "https://go.dev/dl/go${GO_VER}.${GO_OS}-${GO_ARCH}.tar.gz"
    rm -rf "$YKC_HOME/go" && mkdir -p "$YKC_HOME/go" && tar -C "$YKC_HOME/go" --strip-components=1 -xzf "$YKC_HOME/go.tgz"
    rm -f "$YKC_HOME/go.tgz"
  fi
  export PATH="$YKC_HOME/go/bin:$PATH"
fi

# ---------- Rust ----------
export RUSTUP_HOME="$YKC_HOME/rustup"
export CARGO_HOME="$YKC_HOME/cargo"
if ! command -v cargo >/dev/null 2>&1 && [ ! -x "$CARGO_HOME/bin/cargo" ]; then
  echo "── 安裝 rustup（minimal）→ $YKC_HOME ──"
  curl -sSf https://sh.rustup.rs -o "$YKC_HOME/rustup-init.sh"
  sh "$YKC_HOME/rustup-init.sh" -y --profile minimal --default-toolchain stable
  rm -f "$YKC_HOME/rustup-init.sh"
else
  echo "✅ 偵測到 cargo：$(command -v cargo || echo "$CARGO_HOME/bin/cargo")"
fi
export PATH="$CARGO_HOME/bin:$PATH"

# rust-analyzer（官方已不再隨 rustup 分發，改從 GitHub releases 下載）
# 僅供 `make lsp` 展示用；下載失敗不影響其他功能。
# 注意：rustup 1.98 可能殘留一個「壞 proxy」（能 command -v 但執行報 Unknown binary），
#       因此以「能真正執行 --version」為準，否則下載官方二進制覆蓋。
if rust-analyzer --version >/dev/null 2>&1; then
  echo "✅ rust-analyzer：$(rust-analyzer --version 2>/dev/null)"
else
  RA_TARGET=""
  case "$OS/$GO_ARCH" in
    Linux/amd64) RA_TARGET="x86_64-unknown-linux-gnu" ;;
    Linux/arm64) RA_TARGET="aarch64-unknown-linux-gnu" ;;
    Darwin/amd64) RA_TARGET="x86_64-apple-darwin" ;;
    Darwin/arm64) RA_TARGET="aarch64-apple-darwin" ;;
  esac
  if [ -n "$RA_TARGET" ]; then
    mkdir -p "$YKC_HOME/bin"
    if curl -sL "https://github.com/rust-lang/rust-analyzer/releases/latest/download/rust-analyzer-${RA_TARGET}.gz" -o "$YKC_HOME/ra.gz" \
       && gunzip -c "$YKC_HOME/ra.gz" > "$YKC_HOME/bin/rust-analyzer" \
       && chmod +x "$YKC_HOME/bin/rust-analyzer" \
       && [ -s "$YKC_HOME/bin/rust-analyzer" ]; then
      rm -f "$YKC_HOME/ra.gz"
      echo "✅ rust-analyzer 已下載 → $YKC_HOME/bin"
    else
      rm -f "$YKC_HOME/ra.gz" "$YKC_HOME/bin/rust-analyzer"
      echo "⚠️ rust-analyzer 下載失敗（僅影響 make lsp；可稍後手動安裝）"
    fi
  else
    echo "⚠️ 無此平台 rust-analyzer 建置（僅影響 make lsp）"
  fi
fi

echo ""
echo "✅ YKC 環境就緒"
echo "── 下一步（在專案根目錄執行）──"
echo "   make verify-all    ← 一鍵跑全部功能實測"
echo "   make smoke         ← 煙測引擎 + 反欺騙比對"
echo "   make judge         ← L4 除錯閉環"
echo "   make guard-score   ← 信任棘輪（撒謊代理 → T0）"
