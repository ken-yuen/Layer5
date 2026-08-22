package kb

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const (
	manifestVersion     = 1
	maxManifestFileSize = 1 << 20
)

// ImportManifest 是一次版本鎖定 KB import 的可重放 release 證據。它把下載來源
// 的 URL、ETag、Last-Modified、內容 SHA、原子統計、資料集版本和產物 blob SHA
// 分離記錄；其中不含時鐘欄位，故同一來源必然產出同一份 manifest 內容。
type ImportManifest struct {
	ManifestVersion        int    `json:"manifest_version"`
	BlobFormatVersion      uint32 `json:"blob_format_version"`
	RustcVersion           string `json:"rustc_version"`
	ErrorIndexURL          string `json:"error_index_url"`
	ErrorIndexSHA256       string `json:"error_index_sha256"`
	TranslationVersion     string `json:"translation_version,omitempty"`
	ErrorIndexETag         string `json:"error_index_etag,omitempty"`
	ErrorIndexLastModified string `json:"error_index_last_modified,omitempty"`
	ErrorCount             int    `json:"error_count"`
	AtomCount              int    `json:"atom_count"`
	DatasetVersion         string `json:"dataset_version"`
	BlobSHA256             string `json:"blob_sha256"`
	BlobBytes              int64  `json:"blob_bytes"`
}

// Manifest 從一份已編碼 blob 建立其 release manifest。呼叫端可以先用 Blob 取得
// bytes，再以同一份 bytes 寫入檔案，避免「manifest 記的產物」與實際落盤產物不同。
func (r *ImportResult) Manifest(blob []byte) (ImportManifest, error) {
	if r == nil {
		return ImportManifest{}, fmt.Errorf("nil import result")
	}
	if len(blob) == 0 {
		return ImportManifest{}, fmt.Errorf("empty blob")
	}
	sum := sha256.Sum256(blob)
	return ImportManifest{
		ManifestVersion:        manifestVersion,
		BlobFormatVersion:      blobVersion,
		RustcVersion:           r.meta.RustcVersion,
		ErrorIndexURL:          r.meta.ErrorIndexURL,
		ErrorIndexSHA256:       r.meta.ErrorIndexSHA256,
		TranslationVersion:     r.meta.TranslationVersion,
		ErrorIndexETag:         r.sourceETag,
		ErrorIndexLastModified: r.sourceLastModified,
		ErrorCount:             r.errorCount,
		AtomCount:              len(r.atoms),
		DatasetVersion:         versionOf(r.atoms),
		BlobSHA256:             hex.EncodeToString(sum[:]),
		BlobBytes:              int64(len(blob)),
	}, nil
}

// SaveWithManifest 原子寫入 KB blob 及其 manifest。manifestPath 留空時使用
// `<blobPath>.manifest.json`；成功回傳的 manifest 可立即供 CI/release 記錄。
func (r *ImportResult) SaveWithManifest(blobPath, manifestPath string) (ImportManifest, error) {
	blob, err := r.Blob()
	if err != nil {
		return ImportManifest{}, err
	}
	manifest, err := r.Manifest(blob)
	if err != nil {
		return ImportManifest{}, err
	}
	if err := saveBlob(blobPath, blob); err != nil {
		return ImportManifest{}, err
	}
	if manifestPath == "" {
		manifestPath = blobPath + ".manifest.json"
	}
	if err := WriteImportManifest(manifestPath, manifest); err != nil {
		return ImportManifest{}, err
	}
	return manifest, nil
}

// WriteImportManifest 以固定 JSON 格式、原子 rename 寫出 manifest。
func WriteImportManifest(path string, manifest ImportManifest) error {
	if err := manifest.Validate(); err != nil {
		return err
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return writeManifestAtomic(path, data)
}

// ReadImportManifest 讀取並驗證 manifest；未知/損毀輸入不會被默默當成可重放資料。
func ReadImportManifest(path string) (ImportManifest, error) {
	f, err := os.Open(path)
	if err != nil {
		return ImportManifest{}, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxManifestFileSize+1))
	if err != nil {
		return ImportManifest{}, err
	}
	if len(data) > maxManifestFileSize {
		return ImportManifest{}, fmt.Errorf("manifest exceeds %d bytes", maxManifestFileSize)
	}
	var manifest ImportManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return ImportManifest{}, fmt.Errorf("decode manifest: %w", err)
	}
	if err := manifest.Validate(); err != nil {
		return ImportManifest{}, err
	}
	return manifest, nil
}

// Validate 檢查 manifest 的固定格式與所有重放所需欄位。
func (m ImportManifest) Validate() error {
	if m.ManifestVersion != manifestVersion || m.BlobFormatVersion != blobVersion {
		return fmt.Errorf("unsupported manifest/blob format %d/%d", m.ManifestVersion, m.BlobFormatVersion)
	}
	if _, err := normalizeRustcVersion(m.RustcVersion); err != nil {
		return err
	}
	if err := validateIndexURL(m.ErrorIndexURL); err != nil {
		return err
	}
	for name, value := range map[string]string{
		"error_index_sha256": m.ErrorIndexSHA256,
		"blob_sha256":        m.BlobSHA256,
	} {
		if !validSHA256(value) {
			return fmt.Errorf("manifest %s is not a SHA-256 hex digest", name)
		}
	}
	if m.ErrorCount <= 0 || m.AtomCount < m.ErrorCount || !validShortHash(m.DatasetVersion) || m.BlobBytes <= 0 {
		return fmt.Errorf("manifest has invalid counts, dataset version, or blob size")
	}
	if strings.TrimSpace(m.TranslationVersion) == "" {
		return fmt.Errorf("manifest is missing translation_version")
	}
	return nil
}

// ReplayImport 從 manifest 指定的來源重新下載、建庫並逐項核對。只要官方內容、HTTP
// witness、原子圖或 blob 任一項漂移，就回傳錯誤而不把不同資料誤稱為可重放。
func ReplayImport(ctx context.Context, manifest ImportManifest, client *http.Client) (*ImportResult, error) {
	if err := manifest.Validate(); err != nil {
		return nil, err
	}
	result, err := ImportErrorIndex(ctx, ImportOptions{
		RustcVersion:  manifest.RustcVersion,
		ErrorIndexURL: manifest.ErrorIndexURL,
		HTTPClient:    client,
	})
	if err != nil {
		return nil, err
	}
	blob, err := result.Blob()
	if err != nil {
		return nil, err
	}
	actual, err := result.Manifest(blob)
	if err != nil {
		return nil, err
	}
	if err := compareManifestReplay(manifest, actual); err != nil {
		return nil, err
	}
	return result, nil
}

func compareManifestReplay(want, got ImportManifest) error {
	checks := []struct {
		name string
		want string
		got  string
	}{
		{"rustc_version", want.RustcVersion, got.RustcVersion},
		{"error_index_url", want.ErrorIndexURL, got.ErrorIndexURL},
		{"error_index_sha256", want.ErrorIndexSHA256, got.ErrorIndexSHA256},
		{"translation_version", want.TranslationVersion, got.TranslationVersion},
		{"dataset_version", want.DatasetVersion, got.DatasetVersion},
		{"blob_sha256", want.BlobSHA256, got.BlobSHA256},
	}
	if want.ErrorIndexETag != "" {
		checks = append(checks, struct {
			name string
			want string
			got  string
		}{"error_index_etag", want.ErrorIndexETag, got.ErrorIndexETag})
	}
	if want.ErrorIndexLastModified != "" {
		checks = append(checks, struct {
			name string
			want string
			got  string
		}{"error_index_last_modified", want.ErrorIndexLastModified, got.ErrorIndexLastModified})
	}
	for _, check := range checks {
		if check.want != check.got {
			return fmt.Errorf("manifest replay mismatch for %s: want %q, got %q", check.name, check.want, check.got)
		}
	}
	if want.ErrorCount != got.ErrorCount || want.AtomCount != got.AtomCount || want.BlobBytes != got.BlobBytes {
		return fmt.Errorf("manifest replay count/size mismatch: errors %d/%d, atoms %d/%d, bytes %d/%d", want.ErrorCount, got.ErrorCount, want.AtomCount, got.AtomCount, want.BlobBytes, got.BlobBytes)
	}
	return nil
}

func validSHA256(s string) bool {
	if len(s) != sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(s)
	return err == nil
}

func validShortHash(s string) bool {
	if len(s) != 16 {
		return false
	}
	_, err := hex.DecodeString(s)
	return err == nil
}

func writeManifestAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := tmp.Chmod(0o644); err != nil {
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
