package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"ykc/internal/atomicfile"
	"ykc/internal/domain"
	"ykc/internal/eventledger"
	"ykc/internal/precompile"
	"ykc/internal/sandbox"
)

func main() {
	project := flag.String("project", ".", "Rust project directory")
	backend := flag.String("sandbox", "auto", "sandbox backend: auto|docker-runsc|podman-runsc|bwrap|native|docker|podman")
	allowNative := flag.Bool("allow-native", false, "allow trusted native execution when no sandbox is available")
	timeout := flag.Duration("timeout", 3*time.Minute, "timeout per stage")
	fetch := flag.Bool("fetch", true, "run cargo fetch before compile stage")
	locked := flag.Bool("locked", false, "force --locked")
	lockedAuto := flag.Bool("locked-auto", true, "use --locked automatically when Cargo.lock exists")
	tests := flag.Bool("tests-no-run", true, "run cargo test --no-run after cargo check")
	clippy := flag.Bool("clippy", false, "run cargo clippy")
	image := flag.String("image", "rust:1.98", "container image for docker/podman backends")
	jsonOut := flag.Bool("json", true, "print JSON report")
	state := flag.String("state", ".ykc", "state directory for report; empty disables write")
	flag.Parse()

	opt := precompile.DefaultOptions(*project)
	opt.Backend = sandbox.NormalizeBackend(*backend)
	opt.AllowNative = *allowNative
	opt.Timeout = *timeout
	opt.Fetch = *fetch
	opt.Locked = *locked
	opt.LockedAuto = *lockedAuto
	opt.TestsNoRun = *tests
	opt.Clippy = *clippy
	opt.Image = *image
	opt.StateDir = *state

	rep, err := precompile.Run(context.Background(), opt)
	if err != nil {
		fmt.Fprintln(os.Stderr, "ykc-precompile:", err)
		os.Exit(1)
	}
	if *state != "" {
		stateDir := *state
		if !filepath.IsAbs(stateDir) {
			stateDir = filepath.Join(rep.ProjectDir, stateDir)
		}
		b, _ := json.MarshalIndent(rep, "", "  ")
		if err := atomicfile.WriteFileSync(filepath.Join(stateDir, "precompile", "report.json"), append(b, '\n'), 0o644); err != nil {
			fmt.Fprintln(os.Stderr, "ykc-precompile: write report:", err)
			os.Exit(1)
		}
		bridge, err := eventledger.Open(stateDir, "ykc-precompile")
		if err != nil {
			fmt.Fprintln(os.Stderr, "ykc-precompile: open audit bridge:", err)
			os.Exit(1)
		}
		e, err := domain.NewEnvelope(domain.EventPrecompileReport, "", rep.ProjectDir, rep)
		if err != nil {
			fmt.Fprintln(os.Stderr, "ykc-precompile: create audit event:", err)
			os.Exit(1)
		}
		if _, err := bridge.Append(e); err != nil {
			fmt.Fprintln(os.Stderr, "ykc-precompile: append audit event:", err)
			os.Exit(1)
		}
	}
	if *jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(rep)
	} else {
		fmt.Printf("YKC precompile: %s\n", rep.Overall)
		for _, st := range rep.Stages {
			fmt.Printf("- %s: %s errors=%d warnings=%d\n", st.Name, st.Status, st.Diagnostics.Errors, st.Diagnostics.Warnings)
		}
	}
	if rep.Overall == precompile.StatusFailed || rep.Overall == precompile.StatusUnsupported {
		os.Exit(2)
	}
}
