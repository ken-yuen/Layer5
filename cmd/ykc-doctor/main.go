// ykc-doctor — 工具鏈顯性探測（T-21c，見 YKC_22 §1.4）。
// 把 EnsureToolchainPath 的隱式猜測改成顯性、可審計的探測報告：
// 開發者與代理讀到同一份真相；-json 供機器讀；-strict 供生產啟動前置檢查。
//
// 用法：
//
//	ykc-doctor [-json] [-strict]
//	  -json    機器可讀輸出（ykc-doctor/v1 schema）
//	  -strict  cargo/rustc 缺席時以非零退出（生產 fail-fast）
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"runtime"

	"ykc/internal/panel"
	"ykc/internal/toolchain"
)

// probe 是單一工具的探測結果。
type probe struct {
	Name    string `json:"name"`
	Found   bool   `json:"found"`
	Path    string `json:"path,omitempty"`
	Version string `json:"version,omitempty"`
	SHA256  string `json:"sha256,omitempty"`
	Note    string `json:"note,omitempty"`
}

// report 是 ykc-doctor/v1 的完整輸出。
type report struct {
	Schema    string  `json:"schema"`
	OS        string  `json:"os"`
	Arch      string  `json:"arch"`
	Toolchain []probe `json:"toolchain"`
	Sandbox   []probe `json:"sandbox"`
	Strict    bool    `json:"strict"`
	Healthy   bool    `json:"healthy"` // strict 語義：cargo+rustc 皆在
}

func probeTool(name string, versionArgs ...string) probe {
	p := probe{Name: name}
	path, err := exec.LookPath(name)
	if err != nil {
		p.Note = "不在 PATH"
		return p
	}
	p.Found, p.Path = true, path
	if b, err := os.ReadFile(path); err == nil {
		h := sha256.Sum256(b)
		p.SHA256 = hex.EncodeToString(h[:])
	}
	if len(versionArgs) > 0 {
		out, err := exec.Command(name, versionArgs...).Output()
		if err == nil {
			v := string(out)
			if i := indexByte(v, '\n'); i >= 0 {
				v = v[:i]
			}
			p.Version = v
		}
	}
	return p
}

func indexByte(s string, c byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == c {
			return i
		}
	}
	return -1
}

func main() {
	asJSON := flag.Bool("json", false, "機器可讀輸出")
	strict := flag.Bool("strict", false, "cargo/rustc 缺席時非零退出（生產 fail-fast）")
	flag.Parse()

	// 與 serve/panel 同一套路徑補齊邏輯：探測結果 = serve 實際看到的環境。
	panel.EnsureToolchainPath()

	rep := report{Schema: "ykc-doctor/v1", OS: runtime.GOOS, Arch: runtime.GOARCH, Strict: *strict}
	for _, t := range []struct {
		name string
		args []string
	}{
		{"cargo", []string{"--version"}},
		{"rustc", []string{"--version"}},
		{"rust-analyzer", []string{"--version"}},
		{"cargo-audit", []string{"--version"}},
		{"cargo-deny", []string{"--version"}},
		{"go", []string{"version"}},
		{"python3", []string{"--version"}},
	} {
		rep.Toolchain = append(rep.Toolchain, probeTool(t.name, t.args...))
	}
	for _, s := range []string{"bwrap", "runsc", "docker", "podman"} {
		rep.Sandbox = append(rep.Sandbox, probeTool(s))
	}

	// healthy = 裁判核心依賴（cargo + rustc）皆在；與 toolchain.attest 同一判準。
	n := toolchain.NewNative()
	ok, why := n.Available()
	rep.Healthy = ok

	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(rep)
	} else {
		fmt.Println("YKC Doctor — 工具鏈顯性探測")
		fmt.Printf("平台：%s/%s\n\n", rep.OS, rep.Arch)
		printProbes("工具鏈", rep.Toolchain)
		printProbes("沙盒後端", rep.Sandbox)
		if ok {
			if info, err := n.Version(context.Background()); err == nil {
				fmt.Printf("\n握手指紋（toolchain.attest 同源）：\n  %s\n  %s\n", info.Rustc, info.Cargo)
				if info.RustAnalyzer != "" {
					fmt.Printf("  %s\n", info.RustAnalyzer)
				}
			}
		} else {
			fmt.Printf("\n⚠️  Rust 工具鏈不可用：%s\n", why)
		}
	}

	if *strict && !rep.Healthy {
		os.Exit(2)
	}
}

func printProbes(title string, ps []probe) {
	fmt.Println(title + "：")
	for _, p := range ps {
		mark := "✅"
		detail := p.Version
		if detail == "" {
			detail = p.Path
		}
		if !p.Found {
			mark, detail = "－", p.Note
		}
		fmt.Printf("  %s %-14s %s\n", mark, p.Name, detail)
	}
}
