# YKC 容器化方案 — Docker 類替代品深度分析與建議
### （2026-08-21 聯網檢索 ｜ 版次 v1.0）

> 目標：讓 YKC「**保證在任何裝置都能運行**」——同一份 YKC 在 macOS / Windows / Linux / CI / 雲端，以位元組一致的環境運行，且安全隔離代理產生的不可信代碼。
>
> 結論先講：**不要「只選一個替代 Docker 的工具」，而要按 YKC 的三種不同需求分層選型**。單一工具的「非此即彼」是這個問題最大的陷阱。

---

## 1. YKC 的三種需求（決定選型的根本）

| 需求 | 內容 | 關鍵屬性 |
|---|---|---|
| **R1 裁判可重現** | YKC 核心（Go 二進制）+ Rust 工具鏈，環境一致 | 確定性、位元組一致 |
| **R2 不可信代碼隔離** | 編譯代理產生的代碼——**cargo build script / proc-macro 會在編譯期執行任意代碼** | 安全邊界（對抗惡意/失控代碼） |
| **R3 任何裝置** | macOS / Windows / Linux / CI / 雲端皆可跑 | 可攜、免費、無鎖定 |

> 注意 R2 常被忽略：`cargo build` 不是純計算，**build script 是編譯時執行的任意程序**。一個「誠實但 buggy」或「存心惡意」的代理，可以透過 build script 讀取環境變數、外傳 token、搞掛裁判主機。因此 YKC 的編譯沙盒必須是「不可信代碼」級別的隔離，這正是選型的最高優先。

---

## 2. 候選方案全景比較（2026 現況）

### 2.1 容器 runtime 層（R1+R3）

| 方案 | 架構 | 授權/成本 | 冷啟動 | 記憶體 | K8s | 適合 YKC 嗎 |
|---|---|---|---|---|---|---|
| **Docker** | 中央 daemon（root） | Docker Desktop 企業收費 ($9–24/人/月) | ~1.0–1.6s | ~100–150MB daemon | 需轉換工具 | 生態最大，但有 daemon+root+授權成本 |
| **Podman** | 無 daemon、rootless 預設 | Apache-2.0 全免費 | ~0.7–1.1s | 0MB daemon | **原生 pod + `podman generate kube`** | ✅ **首選**（安全/免費/K8s 原生） |
| **containerd + nerdctl** | 極簡 daemon | Apache-2.0 | 最快 | ~30MB | K8s 預設 runtime | 太底層，DX 差，適合 K8s 內部 |

來源：[1](https://scopir.com/posts/docker-vs-podman-2026/)、[2](https://www.kunalganglani.com/blog/docker-vs-podman-2026)、[3](https://middleware.io/blog/podman-vs-docker/)、[4](https://eitt.academy/knowledge-base/docker-vs-podman-vs-containerd-comparison-2026/)

**判定**：R1/R3 用 **Podman**。理由：① 無 daemon、rootless = 裁判更難被容器逃逸波及；② 全免費，無 Docker Desktop 授權鎖定（呼應「任何裝置」）；③ CLI 95% 相容 Docker（`alias docker=podman` 即可），交付物仍可用 `docker compose` 語法；④ 原生 K8s 整合，`podman generate kube` 一鍵產 manifest。Docker 保留為「生態相容」的第二軌道——**同一份 OCI 鏡像 + 同一份 Dockerfile 兩者通用**，所以選 Podman 不會鎖死。

### 2.2 沙盒 runtime 層（R2，安全邊界）

| 方案 | 隔離型別 | 冷啟動 | 記憶體開銷 | CPU 開銷 | 系統呼叫相容 | 適合 YKC 嗎 |
|---|---|---|---|---|---|---|
| **runc（標準 Docker）** | 命名空間（共用宿主核心） | ~20ms | ~7MB | ~0% | 全 | ⚠️ **不足以隔離不可信代碼** |
| **gVisor (runsc)** | 用戶空間核心（Sentry）攔截 syscall | ~50ms | ~18MB | 高（可達 95% 退化，syscall 密集時） | 部分 | ✅ **編譯沙盒首選（ptrace 可攜/kvm 加速）** |
| **Firecracker** | 硬體級 microVM | ~125ms | ~5MB | ~4% | 全 Linux | ✅ **高保障層（雲端多租戶）** |
| **Kata Containers** | VM（QEMU/Firecracker 可選） | ~125–500ms | ~28–52MB | ~4% | 全 | K8s 整合最佳，但較重 |

來源：[5](https://kubernetes.recipes/recipes/security/gvisor-container-runtime/)、[6](https://www.augmentcode.com/guides/agent-execution-sandbox)、[7](https://stackharbor.com/en/knowledge-base/gvisor-sandbox-runtime/)、[8](https://hokstadconsulting.com/blog/how-to-pick-the-right-microvm-orchestrator)、[9](https://github.com/copyleftdev/micro-containers)、[10](https://rywalker.com/research/container-vm-runtimes)

**關鍵業界共識（2026）**：

> 「生產級 AI agent 執行沙盒的最低可接受隔離 = **gVisor 或 Firecracker microVM**；標準 Docker/runc 共用宿主核心，**明確不足以隔離不可信 agent 代碼**。」（[6](https://www.augmentcode.com/guides/agent-execution-sandbox)）

這直接命中 YKC：代理產生的 Rust 專案含 build script，就是「不可信代碼」。**判定**：
- **單機/內測**：gVisor `runsc`（ptrace 平台任何機器可用；有 /dev/kvm 時切 kvm 加速）。~50ms 冷啟動、幾十 MB 開銷，對「編譯專案」這類 CPU 密集但 syscall 不密集的負載可接受。
- **雲端多租戶 SaaS**：Firecracker（硬體邊界、~125ms、~5MB、~4% 開銷，AWS Lambda/Fargate 驗證過）；或 Kata + Firecracker VMM（K8s RuntimeClass 原生）。
- **永不**：在裁判進程內裸跑 `cargo build`（= 讓 build script 在裁判主機任意執行）。

### 2.3 可重現工具鏈層（R1 的進階選項）

| 方案 | 機制 | 價值 | 取捨 |
|---|---|---|---|
| **rust-toolchain.toml** | cargo/rustup 讀取並自動安裝鎖定版本 | 零成本、跨平台、標準做法 | 依賴 rustup 在場 |
| **Nix flake + rust-overlay** | 宣告式鎖定 nixpkgs commit + 精確 rustc | **位元組級**可重現（連 C 庫都鎖） | 學習曲線陡 |
| **devenv / devbox / mise** | Nix 之上的易用層 | 低門檻享受 Nix 可重現 | 多一層抽象 |

來源：[11](https://www.rustfaq.org/en/how-to-use-nix-for-reproducible-rust-builds/)、[12](https://blog.rajpoot.dev/posts/devops/nix-devbox-dev-environments-2026/)

**判定**：主線用 **rust-toolchain.toml 鎖版**（標準、零成本、任何裝置）；**Nix flake 作為「最高確定性」的可選層**，供要求「位元組級審計」的企業客戶。不把 Nix 當預設（門檻會傷害「任何裝置都能跑」的親和力）。

### 2.4 鏡像構建優化（次要但影響體驗）

- **cargo-chef**：把 Rust 依賴層與程式碼層分離，Docker layer 快取命中，構建加速可達數倍 [13](https://earthly.dev/blog/cargo-chef/)（主要用在「YKC 用 YKC 開發自己」時加速）。
- **distroless/scratch**：Go 靜態二進制最終鏡像 ~3–9MB，無 shell、無套件管理器 = 最小攻擊面 [14](https://oneuptime.com/blog/post/2026-02-08-how-to-choose-between-scratch-and-distroless-base-images/view/)、[15](https://dasroot.net/posts/2026/02/building-minimal-go-containers-high-throughput-apis/)。

---

## 3. 最終建議方案（分層，已落地到 deploy/）

```
┌─────────────────────────────────────────────────────────┐
│  T3  雲端 SaaS 多租戶     Kubernetes + RuntimeClass:gvisor│
│       （Firecracker/Kata 可選升級）                       │
├─────────────────────────────────────────────────────────┤
│  T2  編譯沙盒（不可信代碼）  gVisor runsc（單機）/ Firecracker│
│       所有 cargo build 在此執行，禁止在裁判進程內裸跑     │
├─────────────────────────────────────────────────────────┤
│  T1  可重現工具鏈         OCI 鏡像（Podman/Docker 通用）   │
│       rust-toolchain.toml 鎖 rustc 1.98.0 + 各工具鎖版    │
├─────────────────────────────────────────────────────────┤
│  T0  原生裁判             3.3MB 靜態 Go 二進制（零依賴）   │
│       無任何容器也能在 macOS/Win/Linux/CI 直接運行         │
└─────────────────────────────────────────────────────────┘
```

**主選**：Podman（rootless、免費、K8s 原生）跑 YKC 裁判鏡像；**gVisor** 隔離編譯；**rust-toolchain.toml** 鎖工具鏈；**Firecracker/K8s** 作雲端商業版擴張。
**相容性**：所有 Dockerfile/compose 語法對 Docker 同樣有效（`alias docker=podman`），**零鎖定**。

**為什麼這能「保證任何裝置都能運行」**：
1. T0 靜態二進制——無 runtime、無容器、無依賴，直接可執行（已實測 `not a dynamic executable`）。
2. T1 鎖版鏡像——rustc/cargo/rust-analyzer 全部版本鎖定，任何機器 pull 同一鏡像 = 同一環境，消除「我這能跑你那不能跑」。
3. T2/T3 把「不可信代碼」關進真正安全邊界，裁判永不因惡意 build script 而失守。
4. 全免費工具鏈（Podman/gVisor/Firecracker 皆 Apache-2.0），無 Docker Desktop 企業授權鎖定 = 真正「任何裝置、任何規模」。

---

## 4. 落地清單（本輪已交付，見 deploy/）

| 檔案 | 作用 |
|---|---|
| `deploy/Dockerfile` | multi-stage：Go 靜態編譯 → 鎖版 Rust 工具鏈鏡像 → 最小可執行鏡像 |
| `deploy/rust-toolchain.toml` | 鎖 rustc 1.98.0 + rustfmt/clippy/rust-analyzer（任何裝置一致） |
| `deploy/compose.yaml` | 本機「任何裝置」一鍵跑：`docker compose up` |
| `deploy/k8s/deployment.yaml` | K8s 部署裁判（YAML 由 podman generate kube 同款語法） |
| `deploy/k8s/sandbox-runtime.yaml` | gVisor RuntimeClass + 沙盒 Pod 範例（不可信代碼隔離） |
| `deploy/sandbox/README.md` | gVisor / Firecracker 安裝與啟用步驟 |
| `Makefile` | build / image / up / smoke 一鍵命令 |

---

## 5. 風險與緩解

| 風險 | 緩解 |
|---|---|
| gVisor 高 syscall 負載下 CPU 退化 | 編譯是 CPU 密集但 syscall 稀疏；有 /dev/kvm 用 kvm 平台；極端情況升級 Firecracker |
| gVisor 系統呼叫相容不全（部分 crate 的 build script 用冷門 syscall） | 沙盒內跑通才承認 receipt；不相容時自動升 Firecracker（全 Linux syscall 相容） |
| build script 需要網路下載依賴 | 網路 egress 白名單（僅 crates.io/sparse index/git 白名單），禁 metadata endpoint |
| 鏡像體積大（含完整 Rust 工具鏈） | 裁判鏡像與工具鏈鏡像分離；層快取；必要時用 cargo-chef |
| Docker Desktop 授權鎖定 | 全部基於 Podman 相容語法；交付物雙軌可用 |

---

## 6. 一句話總結

**YKC 的「任何裝置可運行」不是靠某一個容器工具，而是靠「靜態二進制（T0）＋鎖版鏡像（T1）＋安全沙盒 runtime（T2）＋K8s（T3）」四層分級；主選 Podman（免費無鎖定）＋ gVisor/Firecracker（不可信代碼邊界）＋ rust-toolchain.toml（位元組一致）。** 這套組合同時滿足可重現、安全隔離、可攜、免費四項，且對 Docker 生態零鎖定。

---

*版次 v1.0 ｜ 資料來源為 2026-08 聯網檢索（引用 [1]–[15]）；落地檔見 `deploy/`。*
