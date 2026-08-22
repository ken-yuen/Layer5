#!/usr/bin/env bash
# YKC 環境一鍵還原（冪等、零 sudo、跨平台：Linux / macOS / Windows-WSL）
#
# 用法：
#   bash dev-setup.sh            # 工具鏈裝到 $HOME/.ykc（預設，無需 sudo）
#   YKC_HOME=/opt/ykc bash dev-setup.sh   # 自訂位置
#
# 行為：
#   - 優先沿用系統 Go；Rust 則固定安裝/使用 $YKC_HOME 的鎖定版本
#   - staticcheck 與 rust-analyzer（版本 + SHA-256）自動補裝，供品質／LSP 驗證
#   - 冪等：可重複執行，不會重複下載
set -e

GO_VER="1.27.0"
RUST_VER="1.98.0"
RA_VER="2026-08-17.4"
STATICCHECK_VER="v0.8.1"
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

# ---------- Go lint 工具 ----------
# staticcheck 不進 YKC 二進制的執行期依賴圖，但它是 make lint / verify-all 的
# 強制品質閘門。固定版本讓本機與 CI 得到同一套診斷規則。
STATICCHECK_BIN="$YKC_HOME/bin/staticcheck"
if [ -x "$STATICCHECK_BIN" ] && "$STATICCHECK_BIN" -version 2>/dev/null | grep -q "(${STATICCHECK_VER#v})"; then
  echo "✅ staticcheck：$($STATICCHECK_BIN -version)"
else
  echo "── 安裝 staticcheck $STATICCHECK_VER → $STATICCHECK_BIN ──"
  mkdir -p "$YKC_HOME/bin"
  GOBIN="$YKC_HOME/bin" go install "honnef.co/go/tools/cmd/staticcheck@${STATICCHECK_VER}"
fi

# ---------- Rust（鎖定 toolchain） ----------
# YKC 會自行維護 $YKC_HOME 下的 rustup/cargo，避免全域 stable 在日後升版後令
# judge、KB error index 與 CI 產生不同診斷。
export RUSTUP_HOME="$YKC_HOME/rustup"
export CARGO_HOME="$YKC_HOME/cargo"
if [ ! -x "$CARGO_HOME/bin/cargo" ]; then
  echo "── 安裝 rustup（minimal，rustc $RUST_VER）→ $YKC_HOME ──"
  curl -sSf https://sh.rustup.rs -o "$YKC_HOME/rustup-init.sh"
  sh "$YKC_HOME/rustup-init.sh" -y --profile minimal --default-toolchain "$RUST_VER"
  rm -f "$YKC_HOME/rustup-init.sh"
else
  echo "✅ 偵測到 YKC cargo：$CARGO_HOME/bin/cargo"
fi
export PATH="$CARGO_HOME/bin:$PATH"
if ! rustup run "$RUST_VER" rustc --version >/dev/null 2>&1; then
  echo "── 補裝鎖定 rustc $RUST_VER → $YKC_HOME ──"
  rustup toolchain install "$RUST_VER" --profile minimal
fi
rustup default "$RUST_VER" >/dev/null
echo "✅ rustc：$(rustc --version)"

# ---------- rust-analyzer（鎖定版本 + SHA-256） ----------
# rust-analyzer 已不隨 rustup 分發。它只供 make lsp 展示用，但仍須鎖定，否則
# 同一份 LSP 診斷測試會隨 GitHub latest 漂移。version marker 令 setup 可冪等。
RA_TARGET=""
RA_SHA256=""
case "$OS/$GO_ARCH" in
  Linux/amd64)
    RA_TARGET="x86_64-unknown-linux-gnu"
    RA_SHA256="a559eaa29920e4c12718fba101f2055f1da0ad8bc458ef9dc1a670778cc66901"
    ;;
  Linux/arm64)
    RA_TARGET="aarch64-unknown-linux-gnu"
    RA_SHA256="941ad31c4256eec3c8457257b0fcfb696d2b4f80c0e5a996f7375a92130c2447"
    ;;
  Darwin/amd64)
    RA_TARGET="x86_64-apple-darwin"
    RA_SHA256="134a7d305991de776864e43d1e6c291f60fa2888d4b9b7749864c562c5dc28b7"
    ;;
  Darwin/arm64)
    RA_TARGET="aarch64-apple-darwin"
    RA_SHA256="ece932daf2f077be87bf745d2eb0a62cbc550f4b1e2e31ca76dfafdd0cc599b3"
    ;;
esac
RA_BIN="$YKC_HOME/bin/rust-analyzer"
RA_MARKER="$YKC_HOME/bin/rust-analyzer.version"
if [ -n "$RA_TARGET" ] && [ -x "$RA_BIN" ] \
   && [ "$(cat "$RA_MARKER" 2>/dev/null || true)" = "$RA_VER" ] \
   && "$RA_BIN" --version >/dev/null 2>&1; then
  echo "✅ rust-analyzer ($RA_VER)：$($RA_BIN --version 2>/dev/null)"
elif [ -n "$RA_TARGET" ]; then
  mkdir -p "$YKC_HOME/bin"
  RA_ARCHIVE="$YKC_HOME/ra-${RA_VER}.gz"
  RA_TMP="$RA_BIN.tmp"
  RA_OK=false
  echo "── 下載 rust-analyzer $RA_VER ($RA_TARGET) → $RA_BIN ──"
  if curl -fsSL "https://github.com/rust-lang/rust-analyzer/releases/download/${RA_VER}/rust-analyzer-${RA_TARGET}.gz" -o "$RA_ARCHIVE"; then
    RA_HASH_OK=false
    if command -v sha256sum >/dev/null 2>&1; then
      if printf '%s  %s\n' "$RA_SHA256" "$RA_ARCHIVE" | sha256sum -c -; then
        RA_HASH_OK=true
      fi
    elif printf '%s  %s\n' "$RA_SHA256" "$RA_ARCHIVE" | shasum -a 256 -c -; then
      RA_HASH_OK=true
    fi
    if [ "$RA_HASH_OK" = true ] \
       && gunzip -c "$RA_ARCHIVE" > "$RA_TMP" \
       && chmod +x "$RA_TMP" \
       && [ -s "$RA_TMP" ] \
       && "$RA_TMP" --version >/dev/null 2>&1; then
      mv -f "$RA_TMP" "$RA_BIN"
      printf '%s\n' "$RA_VER" > "$RA_MARKER"
      RA_OK=true
    fi
  fi
  rm -f "$RA_ARCHIVE" "$RA_TMP"
  if [ "$RA_OK" = true ]; then
    echo "✅ rust-analyzer 已下載並校驗（$RA_VER）→ $YKC_HOME/bin"
  else
    rm -f "$RA_BIN" "$RA_MARKER"
    echo "⚠️ rust-analyzer $RA_VER 下載或 SHA 校驗失敗（僅影響 make lsp；可稍後重跑 setup）"
  fi
else
  echo "⚠️ 無此平台 rust-analyzer 鎖定建置（僅影響 make lsp）"
fi

echo ""
echo "✅ YKC 環境就緒"
echo "── 下一步（在專案根目錄執行）──"
echo "   make verify-all    ← 一鍵跑全部功能實測"
echo "   make smoke         ← 煙測引擎 + 反欺騙比對"
echo "   make judge         ← L4 除錯閉環"
echo "   make guard-score   ← 信任棘輪（撒謊代理 → T0）"
