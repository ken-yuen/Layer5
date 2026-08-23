#!/usr/bin/env bash
# YKC bootstrap-go — go.dev 不可達環境（內網 / 受限 CI / 防火牆沙盒）的 Go 安裝備援。
#
# 原理：golang/go 各 release 的 src/make.bash 明文標註「最低 bootstrap 版本」
# （floor），而 Go 1.4 可用純 C（gcc）從源碼編譯。因此存在一條最短路徑，
# 每一級的 floor 恰為前一級（或更舊）：
#
#   gcc ─► go1.4.3 ─► go1.12.17 ─► go1.17.13 ─► go1.20.14 ─► go1.22.12 ─► go1.24.6 ─► go1.27.0
#        (C 編譯)   (floor ≥1.4)  (floor ≥1.4)  (floor ≥1.17.13) (floor ≥1.20.6) (floor ≥1.22.6) (floor ≥1.24.6)
#
# 源碼自 codeload.github.com 取得（tag tarball），就地編譯（GOROOT 烘焙正確），
# 零 sudo、CGO_ENABLED=0。實測 2C/3GB 機器全鏈約 18 分鐘（2026-08-22）。
#
# 用法：
#   bash bootstrap-go.sh                 # 裝到 $YKC_HOME（預設 ~/.ykc）
#   YKC_HOME=/opt/ykc bash bootstrap-go.sh
#
# 完成後 $YKC_HOME/go 指向 go1.27.0 工具鏈（與 dev-setup.sh 的預期位置一致）。
# 冪等：已建成的級別自動跳過。
set -Eeuo pipefail
YKC_HOME="${YKC_HOME:-$HOME/.ykc}"
LOG="$YKC_HOME/bootstrap-go.log"
mkdir -p "$YKC_HOME"
export CGO_ENABLED=0
export GO111MODULE=off
export GOENV=off

log() { echo "[$(date '+%H:%M:%S')] $*" | tee -a "$LOG"; }

step() {
  local tag=$1 parent=$2
  local ver=${tag#go}
  local dest="$YKC_HOME/go-$ver"
  log "===== $tag (bootstrap: ${parent:-C/gcc}) ====="
  if [ -x "$dest/bin/go" ]; then
    if "$dest/bin/go" version >/dev/null 2>&1 || GOROOT="$dest" "$dest/bin/go" version >/dev/null 2>&1; then
      log "  已存在且可運行 → 跳過"
      return 0
    fi
  fi
  rm -rf "$dest"
  log "  下載 $tag（codeload）..."
  local tmp="$YKC_HOME/$tag.tar.gz"
  curl -sSL --retry 3 --retry-delay 5 -o "$tmp" \
    "https://codeload.github.com/golang/go/tar.gz/refs/tags/$tag" \
    || { log "  FAIL: 下載 $tag 失敗"; return 1; }
  mkdir -p "$dest"
  tar -C "$dest" --strip-components=1 -xzf "$tmp" || { log "  FAIL: 解壓 $tag"; rm -f "$tmp"; return 1; }
  rm -f "$tmp"
  log "  編譯 $tag（就地，日誌 $YKC_HOME/build-$tag.log）..."
  cd "$dest/src" || return 1
  if [ -n "$parent" ]; then
    GOROOT_BOOTSTRAP="$parent" PATH="$parent/bin:$PATH" ./make.bash >"$YKC_HOME/build-$tag.log" 2>&1
  else
    ./make.bash >"$YKC_HOME/build-$tag.log" 2>&1
  fi
  local rc=$?
  if [ $rc -ne 0 ] || [ ! -x "$dest/bin/go" ]; then
    log "  FAIL: 編譯 $tag (rc=$rc)，末 25 行："
    tail -25 "$YKC_HOME/build-$tag.log" | tee -a "$LOG"
    return 1
  fi
  local v
  v=$(GOROOT="$dest" "$dest/bin/go" version 2>&1) || { log "  FAIL: $tag 無法運行: $v"; return 1; }
  log "  OK: $v"
}

# 目標版本與鏈路：升級 Go 時只需改此處（並確認各級 floor 仍成立）。
FINAL=go1.27.0
step go1.4.3    ""
step go1.12.17  "$YKC_HOME/go-1.4.3"
step go1.17.13  "$YKC_HOME/go-1.12.17"
step go1.20.14  "$YKC_HOME/go-1.17.13"
step go1.22.12  "$YKC_HOME/go-1.20.14"
step go1.24.6   "$YKC_HOME/go-1.22.12"
step "$FINAL"   "$YKC_HOME/go-1.24.6"

final="$YKC_HOME/go-${FINAL#go}"
if [ -x "$final/bin/go" ]; then
  rm -rf "$YKC_HOME/go"
  ln -sfn "$final" "$YKC_HOME/go"
  log "完成：$YKC_HOME/go → $final（$("$YKC_HOME/go/bin/go" version)）"
  log "全鏈成功"
else
  log "鏈路不完整：最終工具鏈缺失"
  exit 1
fi
