package ledger

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	urlpkg "net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const (
	anchorVersion      = 1
	anchorKeyBytes     = 32
	maxAnchorFileBytes = 64 * 1024
	witnessTimeout     = 4 * time.Second
)

var (
	// ErrAnchorRollback 表示目前帳本比最後一個獨立 anchor 更短；這是截斷或
	// 回滾的直接證據，寫入路徑必須 fail closed。
	ErrAnchorRollback = errors.New("ledger anchor rollback detected")
	// ErrAnchorMismatch 表示目前鏈在 anchor 的序號處不再包含同一個 hash；這代表
	// 末行重簽、重寫或以另一條合法格式鏈取代原鏈。
	ErrAnchorMismatch = errors.New("ledger anchor mismatch detected")
	// ErrAnchorInvalid 表示 anchor 本身無法驗證（格式、路徑或 HMAC 不正確）。
	ErrAnchorInvalid = errors.New("ledger anchor is invalid")
)

// HeadAnchor 是獨立存放的、已 HMAC 簽章的帳本鏈頭見證。它不放在專案 `.ykc`
// 下，而是預設存於 `$YKC_HOME/anchors`；因此單純取得專案工作目錄寫入權的攻擊者
// 無法透過截斷 ledger 同步回滾它。
type HeadAnchor struct {
	Version    int    `json:"version"`
	LedgerID   string `json:"ledger_id"`
	LedgerPath string `json:"ledger_path"`
	Seq        uint64 `json:"seq"`
	Head       string `json:"head"`
	UpdatedAt  string `json:"updated_at"`
	Signature  string `json:"signature"`
}

// AnchorStatus 是鏈頭錨定的可審計狀態。State 為 anchored、unanchored、
// rollback、mismatch、invalid 或 chain_invalid；unanchored 只允許作為舊帳本
// 第一次升級時的 bootstrap 狀態，下一次成功 Append 會自動建立 anchor。
type AnchorStatus struct {
	State        string `json:"state"`
	LedgerID     string `json:"ledger_id,omitempty"`
	AnchorPath   string `json:"-"` // 僅供本機程式／測試；不經觀察端洩露操作者 home 路徑
	AnchorSeq    uint64 `json:"anchor_seq,omitempty"`
	AnchorHead   string `json:"anchor_head,omitempty"`
	UpdatedAt    string `json:"updated_at,omitempty"`
	CurrentSeq   uint64 `json:"current_seq"`
	CurrentHead  string `json:"current_head"`
	WitnessState string `json:"witness_state,omitempty"` // unconfigured|configured|verified|stale|unavailable|mismatch
	WitnessSeq   uint64 `json:"witness_seq,omitempty"`
	WitnessHead  string `json:"witness_head,omitempty"`
}

type anchorPaths struct {
	ledgerID   string
	ledgerPath string
	anchorPath string
	anchorDir  string
	keyPath    string
}

// AnchorPath 回傳指定帳本的獨立 anchor 檔案路徑。預設根目錄是
// `$YKC_HOME/anchors`（或 `~/.ykc/anchors`）；可由 `YKC_ANCHOR_DIR` 顯式覆寫。
func AnchorPath(ledgerPath string) (string, error) {
	paths, err := resolveAnchorPaths(ledgerPath)
	if err != nil {
		return "", err
	}
	return paths.anchorPath, nil
}

// VerifyAnchored 同時重放 hash chain 與獨立 head anchor。回傳的 bool 只在兩者皆
// 完整時為 true；沒有 anchor 的歷史帳本回傳 true + state=unanchored，讓既有使用者
// 可安全升級，而任何已存在 anchor 的回滾／不一致則回傳明確錯誤。
func VerifyAnchored(path string) (bool, string, AnchorStatus, error) {
	paths, err := resolveAnchorPaths(path)
	if err != nil {
		return false, "", AnchorStatus{State: "invalid"}, err
	}
	anchor, found, err := readAnchor(paths.anchorPath)
	if err != nil {
		return false, "", AnchorStatus{State: "invalid", LedgerID: paths.ledgerID, AnchorPath: paths.anchorPath}, fmt.Errorf("%w: %v", ErrAnchorInvalid, err)
	}
	watchSeq := uint64(0)
	if found {
		watchSeq = anchor.Seq
	}
	scan, err := scanLedger(path, watchSeq)
	status := AnchorStatus{
		LedgerID:    paths.ledgerID,
		AnchorPath:  paths.anchorPath,
		CurrentSeq:  scan.seq,
		CurrentHead: scan.head,
	}
	if err != nil {
		status.State = "chain_invalid"
		return false, scan.head, status, err
	}
	if !scan.valid {
		status.State = "chain_invalid"
		return false, scan.head, status, nil
	}
	if !found {
		status.State = "unanchored"
		return true, scan.head, status, nil
	}
	status.AnchorSeq = anchor.Seq
	status.AnchorHead = anchor.Head
	status.UpdatedAt = anchor.UpdatedAt
	if err := validateAnchor(paths, anchor); err != nil {
		status.State = "invalid"
		return false, scan.head, status, err
	}
	if scan.seq < anchor.Seq {
		status.State = "rollback"
		return false, scan.head, status, fmt.Errorf("%w: current seq %d < anchored seq %d", ErrAnchorRollback, scan.seq, anchor.Seq)
	}
	if !scan.watched || scan.watchedHash != anchor.Head {
		status.State = "mismatch"
		return false, scan.head, status, fmt.Errorf("%w: anchor seq %d no longer maps to anchored head", ErrAnchorMismatch, anchor.Seq)
	}
	status.State = "anchored"
	witnessState, witnessSeq, witnessHead, witnessErr := verifyWitness(paths, anchor)
	status.WitnessState, status.WitnessSeq, status.WitnessHead = witnessState, witnessSeq, witnessHead
	if witnessErr != nil && witnessRequired() {
		return false, scan.head, status, witnessErr
	}
	// 遠端 witness 是可選加強層：未設定／暫時不可用不會削弱已驗證的本機獨立
	// anchor；但一旦取得相互矛盾的已簽章遠端見證，verifyWitness 會直接回傳錯誤。
	if witnessErr != nil && witnessState == "mismatch" {
		return false, scan.head, status, witnessErr
	}
	return true, scan.head, status, nil
}

// writeHeadAnchor 原子更新帳本目前鏈頭的獨立見證。它由 Ledger.Append 在資料行 fsync
// 後呼叫；若這步失敗，Append 會回報錯誤而不允許靜默失去 rollback 防護。
func writeHeadAnchor(ledgerPath string, seq uint64, head string) error {
	if seq == 0 || head == "" || head == Genesis {
		return nil
	}
	paths, err := resolveAnchorPaths(ledgerPath)
	if err != nil {
		return err
	}
	key, err := loadAnchorKey(paths.keyPath, true)
	if err != nil {
		return err
	}
	anchor := HeadAnchor{
		Version:    anchorVersion,
		LedgerID:   paths.ledgerID,
		LedgerPath: paths.ledgerPath,
		Seq:        seq,
		Head:       head,
		UpdatedAt:  time.Now().UTC().Format(time.RFC3339Nano),
	}
	anchor.Signature = signAnchor(key, anchor)
	data, err := json.Marshal(anchor)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(paths.anchorDir, 0o700); err != nil {
		return err
	}
	if err := writePrivateAtomic(paths.anchorPath, data); err != nil {
		return err
	}
	if err := recordWitness(anchor); err != nil && witnessRequired() {
		return fmt.Errorf("required remote witness record failed: %w", err)
	}
	return nil
}

// recordWitness 向可選遠端見證端 POST 已簽章的 anchor。端點協定刻意極小：
// POST <YKC_ANCHOR_WITNESS_URL> 接收 HeadAnchor JSON，成功回 2xx；遠端應以
// ledger_id 單調保存不可變記錄。可用 YKC_ANCHOR_WITNESS_TOKEN 設 Bearer token。
func recordWitness(anchor HeadAnchor) error {
	endpoint, configured, err := witnessEndpoint()
	if err != nil || !configured {
		return err
	}
	body, err := json.Marshal(anchor)
	if err != nil {
		return err
	}
	client := &http.Client{Timeout: witnessTimeout}
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if token := strings.TrimSpace(os.Getenv("YKC_ANCHOR_WITNESS_TOKEN")); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("witness POST returned %s", resp.Status)
	}
	return nil
}

// verifyWitness 在啟用 YKC_ANCHOR_WITNESS_VERIFY（或 required）時讀取遠端最新
// 見證。GET endpoint?ledger_id=<sha256(path)> 應回傳 HeadAnchor JSON，或
// {"anchor": HeadAnchor}。遠端較新的 head 是本機 rollback 證據；同 seq 不同
// head 是重寫證據。網路暫時不可用只標記 unavailable，除非 required=true。
func verifyWitness(paths anchorPaths, local HeadAnchor) (string, uint64, string, error) {
	endpoint, configured, err := witnessEndpoint()
	if err != nil {
		return "unavailable", 0, "", err
	}
	if !configured {
		return "unconfigured", 0, "", nil
	}
	if !witnessVerifyEnabled() {
		return "configured", 0, "", nil
	}
	u, err := urlpkg.Parse(endpoint)
	if err != nil {
		return "unavailable", 0, "", err
	}
	q := u.Query()
	q.Set("ledger_id", paths.ledgerID)
	u.RawQuery = q.Encode()
	client := &http.Client{Timeout: witnessTimeout}
	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return "unavailable", 0, "", err
	}
	if token := strings.TrimSpace(os.Getenv("YKC_ANCHOR_WITNESS_TOKEN")); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := client.Do(req)
	if err != nil {
		return "unavailable", 0, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return "unavailable", 0, "", fmt.Errorf("witness has no record for ledger")
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return "unavailable", 0, "", fmt.Errorf("witness GET returned %s", resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxAnchorFileBytes+1))
	if err != nil {
		return "unavailable", 0, "", fmt.Errorf("read witness response: %w", err)
	}
	if len(body) > maxAnchorFileBytes {
		return "unavailable", 0, "", fmt.Errorf("witness response exceeds %d bytes", maxAnchorFileBytes)
	}
	remote, err := parseWitnessAnchor(body)
	if err != nil {
		return "unavailable", 0, "", err
	}
	if err := validateAnchor(paths, remote); err != nil {
		return "mismatch", remote.Seq, remote.Head, err
	}
	if remote.Seq > local.Seq {
		return "mismatch", remote.Seq, remote.Head, fmt.Errorf("%w: witness seq %d > local anchor seq %d", ErrAnchorRollback, remote.Seq, local.Seq)
	}
	if remote.Seq == local.Seq && remote.Head != local.Head {
		return "mismatch", remote.Seq, remote.Head, fmt.Errorf("%w: witness head differs at seq %d", ErrAnchorMismatch, remote.Seq)
	}
	if remote.Seq < local.Seq {
		return "stale", remote.Seq, remote.Head, nil
	}
	return "verified", remote.Seq, remote.Head, nil
}

func parseWitnessAnchor(body []byte) (HeadAnchor, error) {
	var wrapped struct {
		Anchor *HeadAnchor `json:"anchor"`
	}
	if err := json.Unmarshal(body, &wrapped); err == nil && wrapped.Anchor != nil {
		return *wrapped.Anchor, nil
	}
	var anchor HeadAnchor
	if err := json.Unmarshal(body, &anchor); err != nil {
		return HeadAnchor{}, fmt.Errorf("decode witness anchor: %w", err)
	}
	return anchor, nil
}

func witnessEndpoint() (string, bool, error) {
	raw := strings.TrimSpace(os.Getenv("YKC_ANCHOR_WITNESS_URL"))
	if raw == "" {
		return "", false, nil
	}
	u, err := urlpkg.Parse(raw)
	if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") {
		return "", true, fmt.Errorf("invalid YKC_ANCHOR_WITNESS_URL")
	}
	return raw, true, nil
}

func witnessRequired() bool {
	return envTrue("YKC_ANCHOR_WITNESS_REQUIRED")
}

func witnessVerifyEnabled() bool {
	return witnessRequired() || envTrue("YKC_ANCHOR_WITNESS_VERIFY")
}

func envTrue(key string) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(key))) {
	case "1", "true", "yes", "on", "required":
		return true
	default:
		return false
	}
}

func resolveAnchorPaths(ledgerPath string) (anchorPaths, error) {
	canonical, err := canonicalLedgerPath(ledgerPath)
	if err != nil {
		return anchorPaths{}, err
	}
	sum := sha256.Sum256([]byte(canonical))
	ledgerID := hex.EncodeToString(sum[:])
	ykcHome := strings.TrimSpace(os.Getenv("YKC_HOME"))
	if ykcHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return anchorPaths{}, fmt.Errorf("resolve YKC home for anchor: %w", err)
		}
		ykcHome = filepath.Join(home, ".ykc")
	}
	anchorDir := strings.TrimSpace(os.Getenv("YKC_ANCHOR_DIR"))
	if anchorDir == "" {
		anchorDir = filepath.Join(ykcHome, "anchors")
	}
	keyPath := strings.TrimSpace(os.Getenv("YKC_ANCHOR_KEY_FILE"))
	if keyPath == "" {
		keyPath = filepath.Join(ykcHome, "anchor.key")
	}
	// env 覆寫可為相對路徑；在首次使用時正規化，避免不同 cwd 的 YKC 子程序
	// 對同一帳本寫到不同 anchor/key 位置。
	anchorDir, err = filepath.Abs(anchorDir)
	if err != nil {
		return anchorPaths{}, err
	}
	keyPath, err = filepath.Abs(keyPath)
	if err != nil {
		return anchorPaths{}, err
	}
	return anchorPaths{
		ledgerID:   ledgerID,
		ledgerPath: canonical,
		anchorPath: filepath.Join(anchorDir, ledgerID+".json"),
		anchorDir:  anchorDir,
		keyPath:    keyPath,
	}, nil
}

func canonicalLedgerPath(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		abs = resolved
	}
	return filepath.Clean(abs), nil
}

func readAnchor(path string) (HeadAnchor, bool, error) {
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return HeadAnchor{}, false, nil
	}
	if err != nil {
		return HeadAnchor{}, false, err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, maxAnchorFileBytes+1))
	if err != nil {
		return HeadAnchor{}, false, err
	}
	if len(b) > maxAnchorFileBytes {
		return HeadAnchor{}, false, fmt.Errorf("anchor exceeds %d bytes", maxAnchorFileBytes)
	}
	var anchor HeadAnchor
	if err := json.Unmarshal(b, &anchor); err != nil {
		return HeadAnchor{}, false, fmt.Errorf("decode anchor: %w", err)
	}
	return anchor, true, nil
}

func validateAnchor(paths anchorPaths, anchor HeadAnchor) error {
	if anchor.Version != anchorVersion || anchor.LedgerID != paths.ledgerID || anchor.LedgerPath != paths.ledgerPath || anchor.Seq == 0 || !validFactHash(anchor.Head) || anchor.Signature == "" {
		return fmt.Errorf("%w: metadata does not match ledger", ErrAnchorInvalid)
	}
	key, err := loadAnchorKey(paths.keyPath, false)
	if err != nil {
		return fmt.Errorf("%w: load key: %v", ErrAnchorInvalid, err)
	}
	if !hmac.Equal([]byte(anchor.Signature), []byte(signAnchor(key, anchor))) {
		return fmt.Errorf("%w: HMAC verification failed", ErrAnchorInvalid)
	}
	return nil
}

func validFactHash(s string) bool {
	if len(s) != sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(s)
	return err == nil
}

func signAnchor(key []byte, anchor HeadAnchor) string {
	mac := hmac.New(sha256.New, key)
	_, _ = io.WriteString(mac, anchorSigningPayload(anchor))
	return hex.EncodeToString(mac.Sum(nil))
}

func anchorSigningPayload(anchor HeadAnchor) string {
	return strings.Join([]string{
		strconv.Itoa(anchor.Version), anchor.LedgerID, anchor.LedgerPath,
		strconv.FormatUint(anchor.Seq, 10), anchor.Head, anchor.UpdatedAt,
	}, "\x1f")
}

func loadAnchorKey(path string, create bool) ([]byte, error) {
	b, err := os.ReadFile(path)
	if err == nil {
		if err := requirePrivateFile(path); err != nil {
			return nil, err
		}
		if len(b) != anchorKeyBytes {
			return nil, fmt.Errorf("anchor key has %d bytes, want %d", len(b), anchorKeyBytes)
		}
		return b, nil
	}
	if !errors.Is(err, os.ErrNotExist) || !create {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	key := make([]byte, anchorKeyBytes)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if errors.Is(err, os.ErrExist) {
		return loadAnchorKey(path, false)
	}
	if err != nil {
		return nil, err
	}
	if _, err := f.Write(key); err != nil {
		_ = f.Close()
		return nil, err
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return nil, err
	}
	if err := f.Close(); err != nil {
		return nil, err
	}
	return key, nil
}

func requirePrivateFile(path string) error {
	if runtime.GOOS == "windows" {
		return nil
	}
	fi, err := os.Stat(path)
	if err != nil {
		return err
	}
	if fi.Mode().Perm()&0o077 != 0 {
		return fmt.Errorf("anchor key %s must not be group/world readable", path)
	}
	return nil
}

func writePrivateAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}
