package kb

import (
	"fmt"
	"sort"
	"strings"
)

// DatasetSummary 是 diff 輸出中的資料集身分與來源摘要。
type DatasetSummary struct {
	Version  string      `json:"version"`
	Source   string      `json:"source"`
	Metadata DatasetMeta `json:"metadata"`
	Atoms    int         `json:"atoms"`
}

// AtomRef 是 diff 中足以識別一個上下文原子的精簡視圖。
type AtomRef struct {
	Kind   Kind   `json:"kind"`
	Code   string `json:"code"`
	ID     string `json:"id"`
	Title  string `json:"title"`
	Source string `json:"source,omitempty"`
}

// AtomChange 記錄同一 logical atom（Kind + Code）的內容或知識圖引用改變。Refs
// 不屬於 Atom content ID，因此必須獨立比較，否則知識閉包漂移會被漏報。
type AtomChange struct {
	Kind              Kind   `json:"kind"`
	Code              string `json:"code"`
	Title             string `json:"title"`
	FromID            string `json:"from_id"`
	ToID              string `json:"to_id"`
	ContentChanged    bool   `json:"content_changed"`
	ReferencesChanged bool   `json:"references_changed"`
}

// DiffReport 是兩個版本鎖定 KB blob 的決定論差異報告。
type DiffReport struct {
	Base            DatasetSummary `json:"base"`
	Target          DatasetSummary `json:"target"`
	MetadataChanged bool           `json:"metadata_changed"`
	Added           []AtomRef      `json:"added"`
	Removed         []AtomRef      `json:"removed"`
	Changed         []AtomChange   `json:"changed"`
	Unchanged       int            `json:"unchanged"`
}

// DiffFiles 開啟兩個已校驗 blob，並比較其 metadata、內容定址原子和 graph refs。
func DiffFiles(basePath, targetPath string) (*DiffReport, error) {
	base, err := OpenFile(basePath)
	if err != nil {
		return nil, err
	}
	target, err := OpenFile(targetPath)
	if err != nil {
		return nil, err
	}
	return DiffStores(base, target), nil
}

// DiffStores 計算兩個 Store 的決定論差異。Store 可來自內嵌資料或 blob；只要資料
// 是唯讀且內容定址，輸出就與裝置、map 迭代順序無關。
func DiffStores(base, target *Store) *DiffReport {
	report := &DiffReport{
		Base:    datasetSummary(base),
		Target:  datasetSummary(target),
		Added:   []AtomRef{},
		Removed: []AtomRef{},
		Changed: []AtomChange{},
	}
	report.MetadataChanged = report.Base.Metadata != report.Target.Metadata
	baseByKey := atomsByLogicalKey(base)
	targetByKey := atomsByLogicalKey(target)
	keys := make([]string, 0, len(baseByKey)+len(targetByKey))
	seen := map[string]bool{}
	for key := range baseByKey {
		seen[key] = true
		keys = append(keys, key)
	}
	for key := range targetByKey {
		if !seen[key] {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	for _, key := range keys {
		before, inBase := baseByKey[key]
		after, inTarget := targetByKey[key]
		switch {
		case !inBase:
			report.Added = append(report.Added, atomRef(after))
		case !inTarget:
			report.Removed = append(report.Removed, atomRef(before))
		default:
			contentChanged := before.contentHash() != after.contentHash()
			refsChanged := !sameStringSet(before.Refs, after.Refs)
			if !contentChanged && !refsChanged {
				report.Unchanged++
				continue
			}
			report.Changed = append(report.Changed, AtomChange{
				Kind:              after.Kind,
				Code:              after.Code,
				Title:             after.Title,
				FromID:            before.ID,
				ToID:              after.ID,
				ContentChanged:    contentChanged,
				ReferencesChanged: refsChanged,
			})
		}
	}
	return report
}

// RenderDiffMarkdown 產出可放入 release note 的 bounded、決定論 Markdown。limit
// 為每個變更類別的最大列數；<=0 表示不截斷。
func RenderDiffMarkdown(report *DiffReport, limit int) string {
	if report == nil {
		return "# YKC KB diff\n\n（無報告）\n"
	}
	var sb strings.Builder
	sb.WriteString("# YKC 知識庫版本差異\n\n")
	fmt.Fprintf(&sb, "- base: `%s`（rustc=%s，atoms=%d）\n", report.Base.Version, rustcLabel(report.Base.Metadata), report.Base.Atoms)
	fmt.Fprintf(&sb, "- target: `%s`（rustc=%s，atoms=%d）\n", report.Target.Version, rustcLabel(report.Target.Metadata), report.Target.Atoms)
	fmt.Fprintf(&sb, "- metadata changed: %t\n", report.MetadataChanged)
	fmt.Fprintf(&sb, "- added / removed / changed / unchanged: **%d / %d / %d / %d**\n", len(report.Added), len(report.Removed), len(report.Changed), report.Unchanged)
	if report.MetadataChanged {
		sb.WriteString("\n## Metadata\n\n")
		fmt.Fprintf(&sb, "| field | base | target |\n|---|---|---|\n")
		fmt.Fprintf(&sb, "| rustc | %s | %s |\n", report.Base.Metadata.RustcVersion, report.Target.Metadata.RustcVersion)
		fmt.Fprintf(&sb, "| error index URL | %s | %s |\n", report.Base.Metadata.ErrorIndexURL, report.Target.Metadata.ErrorIndexURL)
		fmt.Fprintf(&sb, "| error index SHA-256 | %s | %s |\n", report.Base.Metadata.ErrorIndexSHA256, report.Target.Metadata.ErrorIndexSHA256)
		fmt.Fprintf(&sb, "| translation | %s | %s |\n", report.Base.Metadata.TranslationVersion, report.Target.Metadata.TranslationVersion)
	}
	renderAtomRefs(&sb, "Added", report.Added, limit)
	renderAtomRefs(&sb, "Removed", report.Removed, limit)
	renderChanges(&sb, report.Changed, limit)
	return sb.String()
}

func datasetSummary(store *Store) DatasetSummary {
	if store == nil {
		return DatasetSummary{}
	}
	return DatasetSummary{Version: store.Version(), Source: store.Source(), Metadata: store.Metadata(), Atoms: store.Count()}
}

func atomsByLogicalKey(store *Store) map[string]*Atom {
	out := map[string]*Atom{}
	if store == nil {
		return out
	}
	for _, atom := range store.atoms {
		out[logicalAtomKey(atom)] = atom
	}
	return out
}

func logicalAtomKey(atom *Atom) string {
	if atom == nil {
		return ""
	}
	return string(atom.Kind) + "\x1f" + strings.ToLower(atom.Code)
}

func atomRef(atom *Atom) AtomRef {
	return AtomRef{Kind: atom.Kind, Code: atom.Code, ID: atom.ID, Title: atom.Title, Source: atom.Source}
}

func sameStringSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	left := append([]string(nil), a...)
	right := append([]string(nil), b...)
	sort.Strings(left)
	sort.Strings(right)
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func rustcLabel(meta DatasetMeta) string {
	if meta.RustcVersion == "" {
		return "legacy/unknown"
	}
	return meta.RustcVersion
}

func renderAtomRefs(sb *strings.Builder, title string, refs []AtomRef, limit int) {
	if len(refs) == 0 {
		return
	}
	fmt.Fprintf(sb, "\n## %s (%d)\n\n", title, len(refs))
	fmt.Fprintln(sb, "| kind | code | atom ID | title |")
	fmt.Fprintln(sb, "|---|---|---|---|")
	for i, ref := range refs {
		if limit > 0 && i == limit {
			fmt.Fprintf(sb, "\n… 另有 %d 筆（以 `-limit 0` 顯示全部）\n", len(refs)-limit)
			break
		}
		fmt.Fprintf(sb, "| %s | %s | `%s` | %s |\n", ref.Kind, ref.Code, ref.ID, strings.ReplaceAll(ref.Title, "|", "\\|"))
	}
}

func renderChanges(sb *strings.Builder, changes []AtomChange, limit int) {
	if len(changes) == 0 {
		return
	}
	fmt.Fprintf(sb, "\n## Changed (%d)\n\n", len(changes))
	fmt.Fprintln(sb, "| kind | code | content | refs | from → to |")
	fmt.Fprintln(sb, "|---|---|---|---|---|")
	for i, change := range changes {
		if limit > 0 && i == limit {
			fmt.Fprintf(sb, "\n… 另有 %d 筆（以 `-limit 0` 顯示全部）\n", len(changes)-limit)
			break
		}
		fmt.Fprintf(sb, "| %s | %s | %t | %t | `%s` → `%s` |\n", change.Kind, change.Code, change.ContentChanged, change.ReferencesChanged, change.FromID, change.ToID)
	}
}
