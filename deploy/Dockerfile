# syntax=docker/dockerfile:1
###############################################################################
# YKC — 裁判鏡像（任何裝置可運行）
#
# 設計原則（見 YKC_01_容器化方案分析.md）：
#   - Stage 1 編譯出「零依賴靜態 Go 二進制」(CGO_ENABLED=0)
#   - Stage 2 以 rust-toolchain.toml 鎖定 Rust 工具鏈版本
#   - 同一份 Dockerfile 對 Podman / Docker 通用（OCI 標準）
#
# 建置：
#   podman build -t ykc:latest -f deploy/Dockerfile .
#   docker build -t ykc:latest -f deploy/Dockerfile .
###############################################################################

# ── Stage 1：編譯 YKC 靜態二進制 ──────────────────────────────────────────
FROM golang:1.27-bookworm AS gobuild
WORKDIR /src
COPY go.mod ./
COPY cmd/ ./cmd/
COPY internal/ ./internal/
COPY core/ ./core/
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/ykc ./cmd/ykc-smoke

# ── Stage 2：鎖版 Rust 工具鏈 ─────────────────────────────────────────────
FROM rust:1.98.0-bookworm AS toolchain

# rust-toolchain.toml 鎖定版本（rustup 會據此安裝；此處再顯式補元件）
COPY deploy/rust-toolchain.toml /tmp/rust-toolchain.toml
RUN rustup component add rustfmt clippy rust-analyzer || true

# T-19 供應鏈工具：固定已做 rustc 1.98 admission 的版本；不得退回 floating latest。
ARG CARGO_AUDIT_VERSION="0.22.2"
ARG CARGO_DENY_VERSION="0.20.2"
RUN cargo install cargo-audit --version "$CARGO_AUDIT_VERSION" --locked \
 && cargo install cargo-deny --version "$CARGO_DENY_VERSION" --locked \
 && cargo audit --version \
 && cargo deny --version

# ── Stage 3：最終可執行鏡像 ───────────────────────────────────────────────
FROM toolchain
COPY --from=gobuild /out/ykc /usr/local/bin/ykc
WORKDIR /workspace
ENTRYPOINT ["ykc"]
