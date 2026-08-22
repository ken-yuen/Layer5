package kb

import "strings"

// DatasetMeta 是知識庫資料集的可追溯中繼資料。它會寫入 v2 blob 的已校驗標頭，
// 讓呼叫端能確認 KB 所依據的 rustc 錯誤索引版本，而非只知道內容雜湊。
type DatasetMeta struct {
	RustcVersion       string `json:"rustc_version,omitempty"`
	ErrorIndexURL      string `json:"error_index_url,omitempty"`
	ErrorIndexSHA256   string `json:"error_index_sha256,omitempty"`
	TranslationVersion string `json:"translation_version,omitempty"`
}

// embeddedDatasetMeta 是隨二進制發布的預設資料集來源。更新 deploy/rust-toolchain.toml
// 時，必須以相同 rustc 版本重新 import 種子資料並同步更新這個常數；kb_test.go
// 會把它與部署鎖檔做一致性檢查。
var embeddedDatasetMeta = DatasetMeta{
	RustcVersion:       "1.98.0",
	ErrorIndexURL:      "https://doc.rust-lang.org/1.98.0/error_codes/print.html",
	TranslationVersion: ErrorTranslationVersion(),
}

// EmbeddedMetadata 回傳隨 YKC 二進制發佈的資料集 metadata 副本。
func EmbeddedMetadata() DatasetMeta { return embeddedDatasetMeta }

// EmbeddedRustcVersion 回傳預設內嵌錯誤索引鎖定的 rustc 版本。
func EmbeddedRustcVersion() string { return embeddedDatasetMeta.RustcVersion }

func normalizeMeta(m DatasetMeta) DatasetMeta {
	m.RustcVersion = strings.TrimSpace(strings.TrimPrefix(m.RustcVersion, "v"))
	m.ErrorIndexURL = strings.TrimSpace(m.ErrorIndexURL)
	m.ErrorIndexSHA256 = strings.ToLower(strings.TrimSpace(m.ErrorIndexSHA256))
	m.TranslationVersion = strings.TrimSpace(m.TranslationVersion)
	return m
}
