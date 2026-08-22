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
//
// v0.0.2 驗測加固（以 7 個真實專案 ~20 萬行掃描驅動）：
//   - 大寫開頭識別符 = 型別/構造子（Ok/Err/Some/Database…）→ 噪音；
//   - 方法/函數呼叫的呼叫名剝除（受者路徑保留: dirs.swap_remove(x) → use dirs, use x）；
//   - 多行結構體字面量等「非翻譯塊」以塊棧配平（不再多發 }）；
//   - 參考的字段寫入 set r.f 降為 use r（引擎不支持 deref coercion）；
//   - 產物陳述數上限 MaxReduceStmts（naive 引擎超線性, 防 OOM）；
//   - closure（任何 |）/else/match 等一律放棄回退（無法忠實表達）。
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

// MaxReduceStmts 是歸約產物的陳述數上限。naive Datalog 引擎對陳述數
// 是超線性複雜度（真實驗測: 大 fn 產物曾令引擎吃盡記憶體被 kill），
// 超限即放棄回退模板。
const MaxReduceStmts = 40

// MaxLoopStmts 是 loop 塊內陳述數上限。後向邊令 reach 閉包稠密化,
// 複雜度遠高於直線代碼（實測: loop 內 16 陳述 ≈5s, 22 ≈20s, 28 超時）,
// 直線 35 陳述 <1s。超限放棄回退模板。
const MaxLoopStmts = 12

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
	reFnLine = regexp.MustCompile(`^\s*(?:pub(?:\([^)]*\))?\s+)?(?:async\s+)?(?:unsafe\s+)?(?:const\s+)?fn\s+(\w+)\s*\(([^)]*)\)`)
	rePath   = regexp.MustCompile(`[A-Za-z_]\w*(?:\.\w+)*`)
	reLetB   = regexp.MustCompile(`^let\s+(?:mut\s+)?(\w+)(?:\s*:[^=]+)?\s*=\s*&\s*(mut\s+)?\*?([A-Za-z_]\w*(?:\.\w+)*)`)
	reLetAny = regexp.MustCompile(`^let\s+(?:mut\s+)?(\w+)`)
	reDrop   = regexp.MustCompile(`^(?:std::mem::)?drop\s*\(\s*([A-Za-z_]\w*(?:\.\w+)*)\s*\)`)
	reAssign = regexp.MustCompile(`^([A-Za-z_]\w*(?:\.\w+)*)\s*(?:=|\+=|-=|\*=|/=)[^=]`)
	reDeref  = regexp.MustCompile(`^\*\s*([A-Za-z_]\w*)\s*(?:=|\+=|-=|\*=|/=)`)
	reReturn = regexp.MustCompile(`^return\b\s*&?\s*\*?([A-Za-z_]\w*(?:\.\w+)*)?`)
	reFor    = regexp.MustCompile(`^for\s+(\w+)\s+in\s+(.*)$`)
	// 放棄線: else/match/closure(任何 |)/impl/async/unsafe/await——
	// 行級啟發法無法忠實表達, 寧退模板（真實驗測: closure 佔比最高）
	reBail = regexp.MustCompile(`\belse\b|\bmatch\b|\bunsafe\b|=>|\bimpl\b|\basync\b|\bawait\b|\|`)
	// 巨集名剝除: name! → 空（其括號內參數仍會做 place 提取）
	reMacro = regexp.MustCompile(`\b\w+!`)
	// 模組路徑前綴剝除: std::mem:: → 空（只留最終段）
	reModPath = regexp.MustCompile(`\b(?:\w+::)+`)
	reAttr    = regexp.MustCompile(`^#!?\[`)
	reIdent   = regexp.MustCompile(`^[A-Za-z_]\w*$`)
)

// rustKeywords: 小寫關鍵字/原生型別（place 提取時忽略; 大寫開頭另由
// isNoiseName 統一判噪——Rust 慣例型別/構造子首字母大寫）。
var rustKeywords = map[string]bool{
	"let": true, "mut": true, "fn": true, "if": true, "else": true,
	"while": true, "for": true, "loop": true, "in": true, "return": true,
	"break": true, "continue": true, "true": true, "false": true,
	"as": true, "ref": true, "where": true, "pub": true, "use": true,
	"mod": true, "crate": true, "super": true, "dyn": true, "static": true,
	"const": true, "enum": true, "struct": true, "trait": true, "type": true,
	"unsafe": true, "async": true, "await": true, "move": true, "impl": true,
	"u8": true, "u16": true, "u32": true, "u64": true, "u128": true,
	"i8": true, "i16": true, "i32": true, "i64": true, "i128": true,
	"usize": true, "isize": true, "f32": true, "f64": true,
	"bool": true, "char": true, "str": true,
}

// isNoiseName: 關鍵字、通配 `_`, 或大寫開頭（型別/構造子/常量: Ok/Err/
// Some/None/String/Database/SCREAMING_CASE…）。`self` 是合法 place。
func isNoiseName(name string) bool {
	if name == "" || name == "_" || rustKeywords[name] {
		return true
	}
	c := name[0]
	return c >= 'A' && c <= 'Z'
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
	used := map[string]bool{}     // 被引用的根名稱（未宣告者 → 補 let）
	isRef := map[string]bool{}    // 名稱是否為參考（借用產物）
	// 塊棧: emitted=是否發射 .cl 塊（{/if {/loop {）; refs=塊內宣告的參考
	// （.cl 塊作用域嚴格——關塊時參考失效, 塊外引用即丟棄, 防 E00 未定義）
	type blockFrame struct {
		emitted bool
		isLoop  bool
		refs    []string
	}
	var blocks []blockFrame
	emittedDepth := 0 // 當前發射塊深度（>0 = 塊內宣告需作用域處理）

	// 參數名（Rust 簽名）預置為已知 place（取冒號前最後一個識別符,
	// 兼容 #[case] x: T 等屬性前綴; 非法識別符跳過）
	for _, p := range strings.Split(fnParams, ",") {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		name := p
		if i := strings.Index(p, ":"); i > 0 {
			name = strings.TrimSpace(p[:i])
		}
		if fs := strings.Fields(name); len(fs) > 0 {
			name = fs[len(fs)-1]
		}
		name = strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(name, "&mut"), "&"))
		if reIdent.MatchString(name) && !isNoiseName(name) {
			used[name] = true
		}
	}

	loopOverflow := false
	loopDepth, loopStmts := 0, 0 // loop 巢內陳述計數（引擎複雜度防線）
	emit := func(cl string, rustLine int) {
		// 相鄰去重（真實代碼多次觸及同名 → 連續 use X 噪音）
		if n := len(out); n > 0 && out[n-1].cl == cl && strings.HasPrefix(cl, "use ") {
			return
		}
		if loopDepth > 0 && cl != "}" {
			loopStmts++
			if loopStmts > MaxLoopStmts {
				loopOverflow = true
			}
		}
		out = append(out, stmt{cl, rustLine})
	}
	openBlock := func(cl string, rustLine int) {
		emit(cl, rustLine)
		blocks = append(blocks, blockFrame{emitted: true, isLoop: cl == "loop {"})
		emittedDepth++
		if cl == "loop {" {
			loopDepth++
		}
	}
	closeBlock := func(rustLine int) {
		if len(blocks) == 0 {
			return
		}
		top := blocks[len(blocks)-1]
		blocks = blocks[:len(blocks)-1]
		if top.emitted {
			emit("}", rustLine)
			emittedDepth--
			if top.isLoop {
				loopDepth--
				if loopDepth == 0 {
					loopStmts = 0
				}
			}
			// 塊內宣告的參考出塊即死（.cl 塊作用域嚴格）: 之後的引用丟棄
			for _, r := range top.refs {
				delete(declared, r)
				delete(isRef, r)
			}
		}
	}
	trackRef := func(name string) {
		if emittedDepth > 0 && len(blocks) > 0 {
			for j := len(blocks) - 1; j >= 0; j-- {
				if blocks[j].emitted {
					blocks[j].refs = append(blocks[j].refs, name)
					return
				}
			}
		}
	}

	// extractPlaces: 從表達式提取真實 place 路徑（呼叫名剝除、噪音過濾）。
	extractPlaces := func(expr string, exclude string) []string {
		expr = reMacro.ReplaceAllString(expr, " ")
		expr = reModPath.ReplaceAllString(expr, " ")
		var found []string
		seen := map[string]bool{}
		locs := rePath.FindAllStringIndex(expr, -1)
		for _, loc := range locs {
			m := expr[loc[0]:loc[1]]
			// 呼叫判定: 路徑後緊跟 ( → 剝除最後一段呼叫名, 保留受者
			j := loc[1]
			for j < len(expr) && expr[j] == ' ' {
				j++
			}
			if j < len(expr) && (expr[j] == '(' || expr[j] == '!') {
				if k := strings.LastIndexByte(m, '.'); k > 0 {
					m = m[:k] // dirs.swap_remove( → dirs
				} else {
					continue // 純函數呼叫名: 丟棄
				}
			}
			root := rootOf(m)
			if isNoiseName(root) || root == exclude || seen[m] {
				continue
			}
			if !declared[root] && !used[root] {
				continue // 未知名稱（外部/綁定變數）: 丟棄
			}
			seen[m] = true
			found = append(found, m)
		}
		return found
	}
	emitUses := func(expr string, rustLine int, exclude string) {
		for _, m := range extractPlaces(expr, exclude) {
			root := rootOf(m)
			if isRef[root] {
				emit("use "+root, rustLine) // 參考: 整名使用（引擎不支持 deref 字段）
			} else {
				emit("use "+m, rustLine)
			}
		}
	}
	// declare: let 綁定名 nm（RHS 已處理後）。
	// - shadowing（同名重 let / 已用名）譯 set（.cl 無 shadowing, set=重初始化）;
	// - 發射塊內的普通宣告**提升**: 頂部 let（used 標記→步驟④）＋原位 set——
	//   .cl 塊作用域嚴格, 原位 let 會令塊外引用 E00 未定義（真實驗測 ripgrep 5 例）。
	declare := func(nm string, rustLine int) {
		switch {
		case declared[nm] || used[nm]:
			if isRef[nm] {
				emit("use "+nm, rustLine) // 參考被 shadow: 近似為使用
				return
			}
			emit("set "+nm, rustLine)
		case emittedDepth > 0:
			used[nm] = true // 提升宣告至頂部
			emit("set "+nm, rustLine)
		default:
			emit("let "+nm, rustLine)
			declared[nm] = true
		}
	}
	// declareRef: 借用宣告 let a = &x（參考名塊作用域追蹤）
	declareRef := func(nm string, rustLine int, cl string) {
		emit(cl, rustLine)
		declared[nm], isRef[nm] = true, true
		trackRef(nm)
	}

	for i := fnStart + 1; i <= fnEnd; i++ {
		raw := strings.TrimSpace(stripStrings(lines[i]))
		raw = stripLineComment(raw)
		// 塊註釋行跳過: /* 開頭, 或 "* " / "*/" 延續行（注意: *b += 1 是解引用, 不可誤殺）
		if raw == "" || raw == ";" || reAttr.MatchString(raw) ||
			strings.HasPrefix(raw, "/*") || strings.HasPrefix(raw, "* ") || strings.HasPrefix(raw, "*/") {
			continue
		}
		ln := i + 1

		// 閉塊: 塊棧配平（fn 自身閉合在 i==fnEnd 且棧空時跳過）
		if strings.HasPrefix(raw, "}") {
			closeBlock(ln)
			// "} else {" 在剝 } 後檢查
			rest := strings.TrimSpace(strings.TrimPrefix(raw, "}"))
			rest = strings.TrimSuffix(rest, ";")
			if rest != "" && reBail.MatchString(rest) {
				return nil, fmt.Errorf("行 %d 含不可歸約結構（else/match/closure…）", ln)
			}
			continue
		}
		if reBail.MatchString(raw) {
			return nil, fmt.Errorf("行 %d 含不可歸約結構（else/match/closure…）", ln)
		}

		opens := strings.HasSuffix(raw, "{")
		content := raw
		if opens {
			content = strings.TrimSpace(strings.TrimSuffix(raw, "{"))
		}
		content = strings.TrimSuffix(content, ";")

		// 塊開啟類（發射 .cl 塊）
		if opens {
			switch {
			case content == "":
				openBlock("{", ln)
				continue
			case strings.HasPrefix(content, "if "):
				emitUses(strings.TrimPrefix(content, "if "), ln, "")
				openBlock("if {", ln)
				continue
			case content == "loop" || strings.HasPrefix(content, "while "):
				if c := strings.TrimPrefix(content, "while "); c != content {
					emitUses(c, ln, "")
				}
				openBlock("loop {", ln)
				continue
			case reFor.MatchString(content):
				m := reFor.FindStringSubmatch(content)
				v, iter := m[1], m[2]
				openBlock("loop {", ln)
				// for x in &mut c / c.iter_mut() → 元素 &mut 借用近似為容器借用
				// （x 與容器同名 = shadowing 自引用, .cl 不支持 → 退化為純使用）
				places := extractPlaces(iter, "")
				if reIdent.MatchString(v) && !isNoiseName(v) && len(places) > 0 &&
					v != rootOf(places[0]) && !isRef[rootOf(places[0])] {
					kind := "&"
					if strings.Contains(iter, "&mut") || strings.Contains(iter, "iter_mut") {
						kind = "&mut "
					}
					declareRef(v, ln, fmt.Sprintf("let %s = %s%s", v, kind, places[0]))
				} else {
					emitUses(iter, ln, "")
				}
				continue
			}
			// 其他行尾 { = 靜默塊（結構體字面量/呼叫換行…）: 內容照譯, 塊不發射
		}

		translateStmt(content, ln, rustcCode, declared, used, isRef,
			emit, emitUses, declare, declareRef, extractPlaces)

		if opens {
			blocks = append(blocks, blockFrame{emitted: false})
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("fn 體無可翻譯陳述")
	}
	if loopOverflow {
		return nil, fmt.Errorf("loop 內產物過大 (>%d 陳述, 防引擎過載)", MaxLoopStmts)
	}

	// ④ 未宣告的根名稱 → 補 let（置頂; 映射行號用 fn 首行; 按名排序保確定性）
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
	nStmts := 0
	for _, s := range all {
		fmt.Fprintf(&b, "  %s\n", s.cl)
		if s.cl != "{" && s.cl != "}" && s.cl != "if {" && s.cl != "loop {" {
			stmtLines = append(stmtLines, s.line)
			nStmts++
		}
	}
	b.WriteString("}\n")
	if nStmts > MaxReduceStmts {
		return nil, fmt.Errorf("產物過大 (%d 陳述 > %d, 防引擎過載)", nStmts, MaxReduceStmts)
	}
	return &Reduction{CL: b.String(), FnName: fnName, StmtLines: stmtLines, SrcFile: srcFile}, nil
}

// translateStmt 翻譯單條（無塊開啟）陳述。
func translateStmt(content string, ln int, rustcCode string,
	declared, used, isRef map[string]bool,
	emit func(string, int), emitUses func(string, int, string),
	declare func(string, int), declareRef func(string, int, string),
	extractPlaces func(string, string) []string) {

	switch {
	case reLetB.MatchString(content): // let a = &(mut) path
		m := reLetB.FindStringSubmatch(content)
		name, mut, place := m[1], strings.TrimSpace(m[2]), m[3]
		// 呼叫受者剝除: &x.method(...) → &x.method 誤捕 → 若後隨 ( 截斷呼叫段
		if idx := strings.Index(content, place) + len(place); idx < len(content) && idx >= 0 {
			rest := strings.TrimLeft(content[idx:], " ")
			if strings.HasPrefix(rest, "(") {
				if k := strings.LastIndexByte(place, '.'); k > 0 {
					place = place[:k]
				} else {
					place = ""
				}
			}
		}
		root := rootOf(place)
		// 不可表達為 .cl 借用的形態, 退化為普通宣告＋使用:
		// - 借用臨時值/構造子（噪音根）
		// - 自借用 let x = &x（.cl 禁 shadowing）
		// - 經參考取字段 &r.field（引擎不支持 deref coercion）
		// - 名已先被使用（used-before-borrow = shadowing 時序, 頂部 let 會與
		//   參考宣告衝突）
		if place == "" || isNoiseName(root) || name == root ||
			(isRef[root] && strings.Contains(place, ".")) ||
			(used[name] && !declared[name]) || declared[name] {
			emitUses(content[strings.Index(content, "=")+1:], ln, name)
			declare(name, ln)
			return
		}
		if !declared[root] {
			used[root] = true
		}
		kind := "&"
		if mut != "" {
			kind = "&mut "
		}
		declareRef(name, ln, fmt.Sprintf("let %s = %s%s", name, kind, place))

	case reDeref.MatchString(content): // *r = / *r +=
		m := reDeref.FindStringSubmatch(content)
		if declared[m[1]] || used[m[1]] {
			emit("use "+m[1], ln)
		}
		if i := strings.IndexByte(content, '='); i >= 0 {
			emitUses(content[i+1:], ln, m[1])
		}

	case reDrop.MatchString(content):
		m := reDrop.FindStringSubmatch(content)
		if declared[rootOf(m[1])] || used[rootOf(m[1])] {
			emit("dp "+m[1], ln)
		}

	case strings.HasPrefix(content, "return"):
		m := reReturn.FindStringSubmatch(content)
		if len(m) > 1 && m[1] != "" && !isNoiseName(rootOf(m[1])) &&
			(declared[rootOf(m[1])] || used[rootOf(m[1])]) {
			if isRef[rootOf(m[1])] {
				emit("ret "+rootOf(m[1]), ln)
				return
			}
			emit("mv "+m[1], ln) // 回傳擁有值 = move 出
			return
		}
		// return Ok(x) / return 複雜式: 構造子剝除後提取內部 place
		emitUses(strings.TrimPrefix(content, "return"), ln, "")

	case reAssign.MatchString(content): // x = … / x.f += …
		m := reAssign.FindStringSubmatch(content)
		target := m[1]
		root := rootOf(target)
		if isNoiseName(root) {
			emitUses(content, ln, "")
			return
		}
		if !declared[root] {
			used[root] = true
		}
		emitUses(content[len(m[0]):], ln, root)
		if isRef[root] {
			emit("use "+root, ln) // set r.f 經參考寫字段: 引擎不支持 → 排他使用近似
		} else {
			emit("set "+target, ln)
		}

	case reLetAny.MatchString(content): // let y = <表達式> / let y
		m := reLetAny.FindStringSubmatch(content)
		name := m[1]
		if name == "_" || strings.HasPrefix(name, "_") && len(name) == 1 {
			// let _ = expr: 通配丟棄——RHS 照譯使用, 不宣告 _
			if i := strings.IndexByte(content, '='); i >= 0 {
				emitUses(content[i+1:], ln, "")
			}
			return
		}
		rhs := ""
		if i := strings.IndexByte(content, '='); i >= 0 {
			rhs = strings.TrimSpace(content[i+1:])
		}
		// 裸標識符 RHS（move 語義候選）
		if reIdent.MatchString(rhs) && !isNoiseName(rhs) && (declared[rhs] || used[rhs]) {
			if moveFamily[rustcCode] && !isRef[rhs] {
				emit("mv "+rhs, ln)
			} else {
				emit("use "+rhs, ln)
			}
			declare(name, ln)
			return
		}
		emitUses(rhs, ln, name)
		declare(name, ln)

	default:
		// 尾表達式（隱式回傳）候選: 單一已知識別符
		c := strings.TrimSuffix(content, ";")
		if reIdent.MatchString(c) && isRef[c] {
			emit("ret "+c, ln)
			return
		}
		emitUses(content, ln, "")
	}
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
	}
	for _, e := range r.Errors {
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
