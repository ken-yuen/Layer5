# YKC 編譯沙盒 runtime 安裝指引（gVisor / Firecracker）

> 為什麼需要：`cargo build` 會執行 build script 與 proc-macro（編譯期任意代碼）。
> 代理產生的代碼屬「不可信代碼」，標準 Docker/runc 共享宿主核心、不足以隔離。
> 2026 業界共識：生產級 agent 沙盒最低標準 = gVisor 或 Firecracker microVM。

## 方案 A：gVisor（單機 / 內測首選，最輕量）

### A.1 安裝（Linux）
```bash
# 下載最新 release 二進制
GVS=$(curl -s https://api.github.com/repos/google/gvisor/releases/latest | jq -r .tag_name)
wget -qO- https://github.com/google/gvisor/releases/download/${GVS}/runsc ${GVS}/runsc.sha512 | head -1
sudo install -m 0755 runsc /usr/local/bin/runsc
```

### A.2 註冊到 Docker / Podman
```bash
# Docker
sudo runsc install -- --runtime-args="--platform=ptrace"   # 無 /dev/kvm 時用 ptrace（最可攜）
# 有 /dev/kvm 時（更快）：
# sudo runsc install -- --runtime-args="--platform=kvm"
sudo systemctl restart docker

# Podman（CRI 之外可經 OCI runtime 接）
# 見 gVisor 官方文件：podman 需以 --runtime /runsc 手動指定
```

### A.3 執行 YKC 沙盒編譯
```bash
# 把「不可信編譯」以 gVisor 執行：
docker run --runtime=runsc --rm -v "$(pwd)/project:/workspace:rw" \
  --network=none --cap-drop=ALL --security-opt=no-new-privileges \
  ykc-toolchain:latest sh -c "cargo build --locked && cargo test"
```

## 方案 B：Firecracker microVM（雲端多租戶 / 高保障）
- 硬體級隔離（~125ms 冷啟動、~5MB 開銷、~4% CPU 開銷），AWS Lambda/Fargate 同款。
- K8s 上以 Kata Containers（Firecracker VMM）RuntimeClass 接入，或經 firecracker-containerd。
- 適用場景：YKC 商業版 SaaS 多租戶、受監管產業客戶。

## 硬性規則（YKC 內部工程規範）
1. **任何「代理產生 / 第三方」代碼的編譯，禁止在裁判進程或標準 runc 容器內裸跑。**
2. 沙盒網路：預設 `--network=none`；需下載依賴時僅放行 crates.io / sparse index / 白名單 git，並封鎖 169.254.169.254（雲 metadata endpoint）。
3. 沙盒每次執行後丟棄（ephemeral），不留可竄改狀態。
4. 只有沙盒內 `cargo build` 成功產出的結果，YKC 才簽發 receipt（T2 執行層，見構圖文件）。
