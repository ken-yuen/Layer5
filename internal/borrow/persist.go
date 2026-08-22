package borrow

// AnalysisReport 是 L5 分析的落盤格式（<project>/.ykc/l5/report.json）。
// 唯一實作：judge 寫（WriteReport）、panel 讀（ReadReport）——消除結構漂移。
// 其 ExplanationSHA256 同步寫入帳本 borrow.analysis 事實，檔案可對賬防竄改。

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// AnalysisReport 是一次 L5 幾何分析的完整落盤記錄。
type AnalysisReport struct {
	Codes             []string        `json:"codes"`
	Reduced           int             `json:"reduced"`   // 真實歸約命中數（量測基礎數據）
	RedEdges          int             `json:"red_edges"` // 違法重疊總數（0 = 幾何收斂）
	L5Available       bool            `json:"l5_available"`
	Graphs            []ConflictGraph `json:"graphs,omitempty"`
	ExplanationSHA256 string          `json:"explanation_sha256"`
	Explanation       string          `json:"explanation"`
	UpdatedAt         string          `json:"updated_at"`
}

func reportPath(projectDir string) string {
	return filepath.Join(projectDir, ".ykc", "l5", "report.json")
}

// WriteReport 原子性弱寫入（先寫後 rename 不必要——單寫者、panel 容忍暫缺）。
func WriteReport(projectDir string, rep AnalysisReport) error {
	d := filepath.Dir(reportPath(projectDir))
	if err := os.MkdirAll(d, 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(rep, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(reportPath(projectDir), b, 0o644)
}

// RemoveReport 清除舊報告（borrow 錯誤全修好時, 防 panel 顯示陳舊紅邊）。
func RemoveReport(projectDir string) { _ = os.Remove(reportPath(projectDir)) }

// ReadReport 讀報告；不存在/損壞回 nil（觀察端容錯, 不影響其他狀態）。
// maxExplain>0 時解釋文本截斷至該長度（展示視圖有界）。
func ReadReport(projectDir string, maxExplain int) *AnalysisReport {
	b, err := os.ReadFile(reportPath(projectDir))
	if err != nil {
		return nil
	}
	var v AnalysisReport
	if json.Unmarshal(b, &v) != nil {
		return nil
	}
	if maxExplain > 0 && len(v.Explanation) > maxExplain {
		v.Explanation = v.Explanation[:maxExplain] + "\n…（截斷; 完整內容見 .ykc/l5/report.json, sha256 對賬帳本）"
	}
	return &v
}
