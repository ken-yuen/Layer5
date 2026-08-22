package borrow

// ============================================================
// 真實 Rust → .cl 歸約（P2, 驗證式啟發法）
//
// 定位與誠實邊界：
//   - 這是「行級啟發式」翻譯，非 AST 級（syn 前端仍是後續工作）；
//   - 因此歸約結果**必須經引擎驗證**才可展示：歸約出的 .cl 要能
//     產生與 rustc 錯誤碼同幾何族的 ChordLaw 錯誤（ChordFamily），
//     驗證不過一律回退 canonical 模板——寧缺勿錯；
//   - 展示時附「sN ← file:line」映射，把幾何拓撲錨回用家源碼。
// ============================================================

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
)

// Reduction 是一次成功的 Rust→.cl 歸約。
type Reduction struct {
	CL        string // 生成的 .cl 源
	FnName    string
	StmtLines []int // 第 i 條 .cl 陳述（s(i+1)）對應的 Rust 行號（1-based）
	SrcFile   string
}

// MaxReduceFnLines 是可歸約的 fn 體行數上限（超過即放棄, 回退模板）。
const MaxReduceFnLines = 60

// chordFamily: rustc 錯誤碼 → 可接受的 ChordLaw 錯誤碼族（驗證判據）。
var chordFamily = map[string][]string{
	"E0499": {"E01", "E09"},
	"E0502": {"E01", "E09"},
	"E0506": {"E02"},
	"E0503": {"E03"},
	"E0505": {"E07", "E08"},
	"E0507": {"E07", "E08", "E06"},
	"E0382": {"E05", "E06"},
	"E0594": {"E05", "E06"},
	"E0597": {"E04", "E05", "E08"},
	"E0716": {"E04", "E05", "E08", "E10"},
	"E0106": {"E10"},
	"E0515": {"E10"},
}

// ChordFamily 回傳 rustc 錯誤碼對應的 ChordLaw 幾何族（無則 nil）。
func ChordFamily(rustcCode string) []string { return chordFamily[rustcCode] }

// moveFamily: 這些碼下, 「let y = x;」的裸標識符 RHS 譯為 mv。
var moveFamily = map[string]bool{
	"E0382": true, "E0505": true, "E0507": true, "E0594": true,
}

var (
	reFnLine  = regexp.MustCompile(`^\s*(?:pub(?:\([^)]*\))?\s+)?(?:async\s+)?(?:unsafe\s+)?fn\s+(\w+)\s*\(([^)]*)\)`)
	rePath    = regexp.MustCompile(`[A-Za-z_]\w*(?:\.\w+)*`)
	reLetB    = regexp.MustCompile(`^let\s+(?:mut\s+)?(\w+)(?:\s*:[^=]+)?\s*=\s*&\s*(mut\s+)?\*?([A-Za-z_]\w*(?:\.\w+)*)`)
	reLetBare = regexp.MustCompile(`^let\s+(?:mut\s+)?(\w+)(?:\s*:[^=]+)?\s*=\s*([A-Za-z_]\w*(?:\.\w+)*)\s*;`)
	reLetAny  = regexp.MustCompile(`^let\s+(?:mut\s+)?(\w+)`)
	reDrop    = regexp.MustCompile(`^(?:std::mem::)?drop\s*\(\s*([A-Za-z_]\w*(?:\.\w+)*)\s*\)`)
	reAssign  = regexp.MustCompile(`^([A-Za-z_]\w*(?:\.\w+)*)\s*(?:=|\+=|-=|\*=|/=)[^=]`)
	reDeref   = regexp.MustCompile(`^\*\s*([A-Za-z_]\w*)\s*(?:=|\+=|-=|\*=|/=)`)
	reReturn  = regexp.MustCompile(`^return\s+&?\s*\*?([A-Za-z_]\w*(?:\.\w+)*)`)
	reBail    = regexp.MustCompile(`\belse\b|\bmatch\b|\bunsafe\b|=>|\bimpl\b|\basync\b|\|\||&&|\bmove\b\s*\|`)
)

// rust 關鍵字/常見非 place 標識符（use 提取時忽略）。
var rustNoise = map[string]bool{
	"let": true, "mut": true, "fn": true, "if": true, "while": true, "for": true,
	"loop": true, "in": true, "return": true, "true": true, "false": true,
	"println": true, "print": true, "vec": true, "String": true, "Some": true,
	"None": true, "Ok": true, "Err": true, "Box": true,
	"std": true, "u32": true, "i32": true, "u64": true, "i64": true, "usize": true,
	"f64": true, "f32": true, "str": true, "new": true, "from": true, "drop": true,
}

// ReduceRust 對 src（整檔 Rust 源）中包含 errLine 的 fn 做行級歸約。
// 失敗（結構太複雜/找不到 fn）回傳 error——呼叫方回退模板。
func ReduceRust(src string, errLine int, rustcCode, srcFile string) (*Reduction, error) {
	lines := strings.Split(src, "\n")
	if errLine < 1 || errLine > len(lines) {
		return nil, fmt.Errorf("錯誤行 %d 超界", errLine)
	}
	// ① 定位包含 errLine 的 fn
	fnStart := -1
	var fnName, fnParams string
	for i := errLine - 1; i >= 0; i-- {
		if m := reFnLine.FindStringSubmatch(lines[i]); m != nil {
			fnStart, fnName, fnParams = i, m[1], m[2]
			break
		}
	}
	if fnStart < 0 {
		return nil, fmt.Errorf("找不到包含錯誤的 fn")
	}
	// ② 括號配對找 fn 結束
	depth, fnEnd := 0, -1
	for i := fnStart; i < len(lines); i++ {
		for _, c := range stripStrings(lines[i]) {
			switch c {
			case '{':
				depth++
			case '}':
				depth--
				if depth == 0 {
					fnEnd = i
				}
			}
		}
		if fnEnd >= 0 {
			break
		}
	}
	if fnEnd < 0 || fnEnd <= fnStart {
		return nil, fmt.Errorf("fn 括號不配對")
	}
	if fnEnd-fnStart > MaxReduceFnLines {
		return nil, fmt.Errorf("fn 過長 (%d 行 > %d)", fnEnd-fnStart, MaxReduceFnLines)
	}
	if errLine-1 < fnStart || errLine-1 > fnEnd {
		return nil, fmt.Errorf("錯誤行不在 fn 內")
	}

	// ③ 逐行翻譯
	type stmt struct {
		cl   string
		line int // rust 行號 1-based
	}
	var out []stmt
	declared := map[string]bool{} // .cl 內已 let 的名稱
	used := map[string]bool{}     // 被引用的根名稱（未宣告者 → fn 參數）
	isRef := map[string]bool{}    // 名稱是否為參考（借用產物）

	// 參數名（Rust 簽名）預置為已知 place
	for _, p := range strings.Split(fnParams, ",") {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		name := p
		if i := strings.Index(p, ":"); i > 0 {
			name = strings.TrimSpace(p[:i])
		}
		name = strings.TrimPrefix(strings.TrimPrefix(name, "&mut "), "&")
		name = strings.TrimSpace(strings.TrimPrefix(name, "mut "))
		if name != "" && !rustNoise[name] && rePath.MatchString(name) {
			used[rootOf(name)] = true
		}
	}

	emit := func(cl string, rustLine int) { out = append(out, stmt{cl, rustLine}) }
	// emitUses: 對表達式內每個已知名稱發 use（去重, 保出現序）
	emitUses := func(expr string, rustLine int, except map[string]bool) {
		seen := map[string]bool{}
		for _, m := range rePath.FindAllString(expr, -1) {
			root := rootOf(m)
			if rustNoise[root] || seen[m] || (except != nil && except[m]) {
				continue
			}
			if declared[root] || used[root] {
				seen[m] = true
				if isRef[root] {
					emit("use "+root, rustLine) // 參考: 整名使用
				} else {
					emit("use "+m, rustLine) // place: 可帶字段路徑
				}
			}
		}
	}

	for i := fnStart + 1; i <= fnEnd; i++ {
		raw := strings.TrimSpace(stripStrings(lines[i]))
		raw = stripLineComment(raw)
		if raw == "" || raw == ";" {
			continue
		}
		ln := i + 1
		if reBail.MatchString(raw) {
			return nil, fmt.Errorf("行 %d 含不可歸約結構（else/match/closure…）", ln)
		}
		switch {
		case i == fnEnd && raw == "}":
			// fn 結束括號, 由模板統一補
		case reLetB.MatchString(raw): // let a = &(mut) place
			m := reLetB.FindStringSubmatch(raw)
			name, mut, place := m[1], strings.TrimSpace(m[2]), m[3]
			if rustNoise[rootOf(place)] {
				return nil, fmt.Errorf("行 %d 借用對象無法識別", ln)
			}
			if !declared[rootOf(place)] {
				used[rootOf(place)] = true
			}
			kind := "&"
			if mut != "" {
				kind = "&mut "
			}
			emit(fmt.Sprintf("let %s = %s%s", name, kind, place), ln)
			declared[name], isRef[name] = true, true
		case reLetBare.MatchString(raw): // let y = x;（裸標識符）
			m := reLetBare.FindStringSubmatch(raw)
			name, rhs := m[1], m[2]
			root := rootOf(rhs)
			if declared[root] || used[root] {
				if moveFamily[rustcCode] && !isRef[root] {
					emit("mv "+rhs, ln) // move 族: 譯為 move
				} else if isRef[root] {
					emit("use "+root, ln)
				} else {
					emit("use "+rhs, ln)
				}
			}
			emit("let "+name, ln)
			declared[name] = true
		case reDeref.MatchString(raw): // *r = / *r +=
			m := reDeref.FindStringSubmatch(raw)
			emit("use "+m[1], ln)
			emitUses(raw[strings.Index(raw, "=")+1:], ln, map[string]bool{m[1]: true})
		case reDrop.MatchString(raw):
			m := reDrop.FindStringSubmatch(raw)
			emit("dp "+m[1], ln)
		case reReturn.MatchString(raw):
			m := reReturn.FindStringSubmatch(raw)
			emit("ret "+rootOf(m[1]), ln)
		case reAssign.MatchString(raw): // x = … / x.f += …
			m := reAssign.FindStringSubmatch(raw)
			target := m[1]
			if rustNoise[rootOf(target)] {
				emitUses(raw, ln, nil)
				break
			}
			if !declared[rootOf(target)] {
				used[rootOf(target)] = true
			}
			rhs := raw[len(m[0]):]
			emitUses(rhs, ln, nil)
			emit("set "+target, ln)
		case reLetAny.MatchString(raw): // let y = <複雜表達式>
			m := reLetAny.FindStringSubmatch(raw)
			emitUses(raw[len(m[0]):], ln, nil)
			emit("let "+m[1], ln)
			declared[m[1]] = true
		case strings.HasPrefix(raw, "if ") && strings.HasSuffix(raw, "{"):
			emitUses(strings.TrimSuffix(strings.TrimPrefix(raw, "if "), "{"), ln, nil)
			emit("if {", ln)
		case (strings.HasPrefix(raw, "loop") || strings.HasPrefix(raw, "while ") ||
			strings.HasPrefix(raw, "for ")) && strings.HasSuffix(raw, "{"):
			emit("loop {", ln)
		case raw == "{":
			emit("{", ln)
		case raw == "}" || raw == "};":
			emit("}", ln)
		default:
			// 尾表達式（隱式 return）: 裸名稱
			if i == fnEnd-1 && rePath.MatchString(raw) && rePath.FindString(raw) == strings.TrimSuffix(raw, ";") {
				name := rootOf(strings.TrimSuffix(raw, ";"))
				if isRef[name] {
					emit("ret "+name, ln)
					continue
				}
			}
			emitUses(raw, ln, nil)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("fn 體無可翻譯陳述")
	}

	// ④ 未宣告的根名稱 → 補 let（宣告置頂; .cl 參數語義=ROOT, 但錯誤現場
	//    多為局部變數, 置頂 let 更接近; 映射行號用 fn 首行; 按名排序保確定性）
	var undeclared []string
	for name := range used {
		if !declared[name] {
			undeclared = append(undeclared, name)
		}
	}
	sort.Strings(undeclared)
	var head []stmt
	for _, name := range undeclared {
		head = append(head, stmt{"let " + name, fnStart + 1})
	}

	// ⑤ 組裝 .cl
	var b strings.Builder
	var stmtLines []int
	fmt.Fprintf(&b, "fn %s() {\n", fnName)
	all := append(head, out...)
	for _, s := range all {
		fmt.Fprintf(&b, "  %s\n", s.cl)
		if s.cl != "{" && s.cl != "}" && s.cl != "if {" && s.cl != "loop {" {
			stmtLines = append(stmtLines, s.line)
		}
	}
	b.WriteString("}\n")
	return &Reduction{CL: b.String(), FnName: fnName, StmtLines: stmtLines, SrcFile: srcFile}, nil
}

// ValidateReduction 用引擎驗證歸約：.cl 必須 FAIL 且錯誤碼落在該
// rustc 碼的幾何族內。回傳 (報告, 是否通過)。
func ValidateReduction(ctx context.Context, a *Analyzer, workDir string, red *Reduction, rustcCode string) (*Report, bool) {
	fam := ChordFamily(rustcCode)
	if len(fam) == 0 {
		return nil, false
	}
	r, err := a.AnalyzeSource(ctx, workDir, red.CL)
	if err != nil || r.Verdict == "PASS" {
		return nil, false
	}
	famSet := map[string]bool{}
	for _, c := range fam {
		famSet[c] = true
	}
	for _, e := range r.Errors {
		if e.Code == "E00" {
			return nil, false // 歸約產物不合語法: 直接否決
		}
		if famSet[e.Code] {
			return r, true
		}
	}
	return nil, false
}

// ReduceFromFile 讀源檔並歸約（judge 掛鉤入口）。
func ReduceFromFile(path string, errLine int, rustcCode string) (*Reduction, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(b) > 256*1024 {
		return nil, fmt.Errorf("源檔過大")
	}
	return ReduceRust(string(b), errLine, rustcCode, path)
}

// MappingText 渲染 sN ← file:line 對照表。
func (r *Reduction) MappingText() string {
	var b strings.Builder
	b.WriteString("陳述 ↔ 源碼行對照:\n")
	for i, ln := range r.StmtLines {
		fmt.Fprintf(&b, "  s%d ← %s:%d\n", i+1, r.SrcFile, ln)
	}
	return b.String()
}

// ---------- 小工具 ----------

// stripStrings 把字串/字元字面量替換為空（避免誤匹配）。
func stripStrings(s string) string {
	var b strings.Builder
	inStr, inChar := false, false
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case inStr:
			if c == '\\' {
				i++
			} else if c == '"' {
				inStr = false
			}
		case inChar:
			if c == '\\' {
				i++
			} else if c == '\'' {
				inChar = false
			}
		case c == '"':
			inStr = true
		case c == '\'' && i+2 < len(s) && (s[i+1] == '\\' || (i+2 < len(s) && s[i+2] == '\'')):
			inChar = true // 字元字面量 'x' / '\n'（生命週期 'a 不觸發）
		default:
			b.WriteByte(c)
		}
	}
	return b.String()
}

func stripLineComment(s string) string {
	if i := strings.Index(s, "//"); i >= 0 {
		return strings.TrimSpace(s[:i])
	}
	return s
}

func rootOf(path string) string {
	if i := strings.IndexByte(path, '.'); i > 0 {
		return path[:i]
	}
	return path
}
