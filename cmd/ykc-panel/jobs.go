// 任務管理器：人類觸發的「控制層」——揀專案、啟動/停止 YKC 動作、實時日誌。
// 與「唯讀觀察」分離：觀察永不改寫；控制由人類明確按下才執行。
package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const logCap = 64 * 1024 // 每任務保留最近 64KB 日誌

// Job 是一次 YKC 動作的執行實例。
type Job struct {
	ID       string    `json:"id"`
	Project  string    `json:"project"`
	Action   string    `json:"action"`
	Claims   string    `json:"claims,omitempty"`
	Command  string    `json:"command"`
	Started  time.Time `json:"started"`
	Status   string    `json:"status"` // running|done|failed|stopped|error
	ExitCode int       `json:"exit_code"`
	Log      string    `json:"log"`

	mu     sync.Mutex
	cmd    *exec.Cmd
	logBuf []byte
}

func (j *Job) appendLog(b []byte) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.logBuf = append(j.logBuf, b...)
	if len(j.logBuf) > logCap {
		j.logBuf = j.logBuf[len(j.logBuf)-logCap:]
	}
}

func (j *Job) snapshot() Job {
	j.mu.Lock()
	defer j.mu.Unlock()
	return Job{
		ID: j.ID, Project: j.Project, Action: j.Action, Claims: j.Claims,
		Command: j.Command, Started: j.Started, Status: j.Status, ExitCode: j.ExitCode,
		Log: string(j.logBuf),
	}
}

type jobWriter struct{ j *Job }

func (w jobWriter) Write(p []byte) (int, error) { w.j.appendLog(p); return len(p), nil }

// JobManager 管理並行執行中的任務。
type JobManager struct {
	mu     sync.Mutex
	jobs   map[string]*Job
	order  []string
	bindir string
}

func newJobManager(bindir string) *JobManager {
	return &JobManager{jobs: map[string]*Job{}, bindir: bindir}
}

func newID() string {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// argv 把「動作名」映對到實際二進制與參數。
func (m *JobManager) argv(action, project, claims string) (string, []string, error) {
	bin := func(n string) string { return filepath.Join(m.bindir, n) }
	switch action {
	case "smoke":
		args := []string{"-dir", project}
		if claims != "" {
			args = append(args, "-claims", claims)
		}
		return bin("ykc"), args, nil
	case "judge":
		return bin("ykc-judge"), []string{"-dir", project}, nil
	case "gate":
		return bin("ykc-judge"), []string{"-dir", project, "-gate"}, nil
	case "verify":
		return bin("ykc-judge"), []string{"-dir", project, "-verify"}, nil
	case "guard-verify":
		if claims == "" {
			return "", nil, fmt.Errorf("guard-verify 需要 claims 檔案路徑")
		}
		return bin("ykc-guard"), []string{"verify", "-dir", project, "-claims", claims}, nil
	case "guard-score":
		if claims == "" {
			return "", nil, fmt.Errorf("guard-score 需要 claims 檔案路徑")
		}
		return bin("ykc-guard"), []string{"score", "-dir", project, "-claims", claims}, nil
	default:
		return "", nil, fmt.Errorf("未知動作: %s", action)
	}
}

// Start 啟動一個任務（非阻塞，立即回傳）。
func (m *JobManager) Start(action, project, claims string) (*Job, error) {
	name, args, err := m.argv(action, project, claims)
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(name); err != nil {
		return nil, fmt.Errorf("找不到可執行檔 %s（請先執行 make binaries）", name)
	}
	cmd := exec.Command(name, args...)
	cmd.Dir = project
	j := &Job{
		ID: newID(), Project: project, Action: action, Claims: claims,
		Command: name + " " + strings.Join(args, " "),
		Started: time.Now().UTC(), Status: "running", cmd: cmd,
	}
	cmd.Stdout = jobWriter{j}
	cmd.Stderr = jobWriter{j}
	if err := cmd.Start(); err != nil {
		j.Status = "error"
		j.appendLog([]byte("啟動失敗: " + err.Error() + "\n"))
	}
	m.mu.Lock()
	m.jobs[j.ID] = j
	m.order = append(m.order, j.ID)
	m.mu.Unlock()
	go func() {
		_ = cmd.Wait()
		j.mu.Lock()
		if j.Status == "stopped" {
			// Stop() 已標記
		} else if cmd.ProcessState != nil {
			if cmd.ProcessState.Success() {
				j.Status, j.ExitCode = "done", 0
			} else {
				j.Status, j.ExitCode = "failed", cmd.ProcessState.ExitCode()
			}
		}
		j.mu.Unlock()
	}()
	return j, nil
}

// Stop 停止任務（僅當仍在運行）。
func (m *JobManager) Stop(id string) error {
	m.mu.Lock()
	j := m.jobs[id]
	m.mu.Unlock()
	if j == nil {
		return fmt.Errorf("任務不存在")
	}
	j.mu.Lock()
	if j.Status != "running" {
		j.mu.Unlock()
		return fmt.Errorf("任務已結束")
	}
	j.Status = "stopped"
	j.mu.Unlock()
	_ = j.cmd.Process.Kill()
	return nil
}

// List 回傳全部任務（新→舊）。
func (m *JobManager) List() []Job {
	m.mu.Lock()
	ids := append([]string(nil), m.order...)
	byID := map[string]*Job{}
	for k, v := range m.jobs {
		byID[k] = v
	}
	m.mu.Unlock()
	out := make([]Job, 0, len(ids))
	for i := len(ids) - 1; i >= 0; i-- {
		if j, ok := byID[ids[i]]; ok {
			out = append(out, j.snapshot())
		}
	}
	return out
}
