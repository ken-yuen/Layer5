// Package datalog 是 YKC 的迷你 Datalog 引擎（零外部依賴）。
//
// 為何自帶引擎：YKC 的「裁判核心」維持零依賴紀律（可信裁判自身供應鏈最小化）；
// 護欄規則集規模有界（數十條規則、數萬事實以內），naive 不動點求值完全夠用，
// 換取「規則即數據」（可審計、可差分、可由用家覆寫）的宣告式語意。
//
// 語法（與 l5/chordlaw/*.dl 同一風格，全專案一致）：
//
//	% 註釋
//	reach(A,B) :- edge(A,B).                 % 正規則
//	edangle(S) :- usep(S,P), !inside(N,M).   % 分層否定（! 前綴）
//	conflict(X,Y) :- lend(X), lend(Y), neq(X,Y).
//
//	慣例：變數大寫開頭；常數小寫開頭、整數、或雙引號字串（可含空格/逗號）；
//	"_" 為匿名變數（每次出現皆新）；內建謂詞 neq(A,B)（常數不等）。
//
// 安全限制（全部顯式報錯，不靜默）：
//   - Datalog 安全性：規則頭的每個變數必須出現於某個正 body 原子；
//   - 否定原子內的變數必須已由其前的正原子綁定（stratifiable 檢查的必要條件）；
//   - 分層否定：負依賴形成環 → 明確報錯（不允許非分層程式）；
//   - 求值有界：MaxDerivedFacts / MaxRounds 超限即報錯（防規則炸裂）。
//
// 決定論：Derived 回傳的事實按（謂詞、項）字典序排序，且每個推導事實攜帶
// 「首次推導它的規則宣告序號」——同一輸入永遠同一輸出（裁判可重放對賬）。
package datalog

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

// ---------- 詞法與語法 ----------

// TermKind 區分常數與變數。
type TermKind int

const (
	Const TermKind = iota
	Var
)

// Term 是一個項。常數一律以字串保存（整數保留原文，比較語意等同）。
type Term struct {
	Kind TermKind
	Val  string
}

// Atom 是謂詞應用：pred(t1, t2, ...)。arity 以規則集內首次出現為準。
type Atom struct {
	Pred  string
	Terms []Term
}

// BodyAtom 是規則體中的一個原子；Negated 為 true 時以 ! 前綴出現。
type BodyAtom struct {
	Atom
	Negated bool
}

// Rule 是 head :- body（body 為合取）。
type Rule struct {
	Head    Atom
	Body    []BodyAtom
	Index   int // 宣告序（0 起）；推導事實攜帶首個推導規則的序號，供穩定排序
	SrcLine int // 1 起，錯誤定位用
}

// Program 是事實（EDB）+ 規則（IDB）的整體。
type Program struct {
	facts  map[string]map[string]factEntry // pred → key → entry（去重）
	arity  map[string]int
	rules  []*Rule
	bounds Bounds
}

// factEntry 記錄一條 ground 事實與其首個推導規則（EDB 為 -1）。
type factEntry struct {
	tuple   []string
	ruleIdx int
}

// Bounds 是求值保險絲（預設 DefaultBounds）。
type Bounds struct {
	MaxDerivedFacts int
	MaxRounds       int
}

// DefaultBounds：護欄規模下的寬鬆上限（觸頂即程式錯誤，顯式報錯）。
var DefaultBounds = Bounds{MaxDerivedFacts: 200_000, MaxRounds: 10_000}

// NewProgram 建立空程式。
func NewProgram() *Program {
	return &Program{
		facts:  map[string]map[string]factEntry{},
		arity:  map[string]int{},
		bounds: DefaultBounds,
	}
}

// SetBounds 覆寫保險絲（僅測試/特殊場景用）。
func (p *Program) SetBounds(b Bounds) { p.bounds = b }

// ---------- 事實 ----------

// AddFact 加入一條 ground 事實（全部參數為常數字串）。重複加入為冪等。
func (p *Program) AddFact(pred string, consts ...string) error {
	if err := validPred(pred); err != nil {
		return err
	}
	if err := p.fixArity(pred, len(consts)); err != nil {
		return err
	}
	p.addGround(pred, consts, -1)
	return nil
}

func (p *Program) addGround(pred string, tuple []string, ruleIdx int) {
	set := p.facts[pred]
	if set == nil {
		set = map[string]factEntry{}
		p.facts[pred] = set
	}
	key := tupleKey(tuple)
	if _, exists := set[key]; !exists {
		set[key] = factEntry{tuple: append([]string(nil), tuple...), ruleIdx: ruleIdx}
	}
}

// ---------- 規則解析 ----------

// ParseRules 解析 .dl 文本為規則（不含事實；事實一律由程式注入）。
// 支援跨行規則：語句以「引號外、其後為空白/行尾/EOF 的句點」終結
// （'claim.kind' 形式的謂詞句點後緊接識別字元，不會被誤判為終結符）。
func ParseRules(src string) ([]*Rule, error) {
	var rules []*Rule
	for _, st := range splitStatements(src) {
		text := strings.TrimSpace(st.text)
		if text == "" {
			continue
		}
		r, err := parseRule(text, st.line)
		if err != nil {
			return nil, err
		}
		rules = append(rules, r)
	}
	return rules, nil
}

// statement 是一段完整規則文本及其起始行號（1 起）。
type statement struct {
	text string
	line int
}

// splitStatements 引號感知地把源文本切成「以 . 結尾」的語句。
func splitStatements(src string) []statement {
	// 先逐行剝註釋，保留行號。
	lines := strings.Split(src, "\n")
	cleaned := make([]string, len(lines))
	for i, l := range lines {
		cleaned[i] = stripComment(l)
	}
	var out []statement
	startLine := 1
	var buf []byte
	for i, l := range cleaned {
		line := l
		for len(line) > 0 {
			// 找本行內（引號外）的第一個終結句點。
			j, sawTerm := scanTerminator(line)
			if sawTerm {
				buf = append(buf, line[:j+1]...)
				out = append(out, statement{text: string(buf), line: startLine})
				buf = nil
				line = line[j+1:]
				startLine = i + 2 // 下一語句最早始於下一行
				continue
			}
			buf = append(buf, line...)
			buf = append(buf, '\n')
			break
		}
		if strings.TrimSpace(l) == "" && len(buf) == 0 {
			startLine = i + 2
		}
	}
	if strings.TrimSpace(string(buf)) != "" {
		out = append(out, statement{text: string(buf), line: startLine}) // 無終結符 → 由 parseRule 報錯
	}
	return out
}

// scanTerminator 回傳本行（引號外）第一個終結句點的位置索引與是否找到。
// 終結句點 = '.' 且（行尾、或其後是空白）——與 tokenizer 的 ident-內句點規則互補。
func scanTerminator(line string) (int, bool) {
	inQuote := byte(0)
	for i := 0; i < len(line); i++ {
		c := line[i]
		if inQuote != 0 {
			if c == inQuote {
				inQuote = 0
			}
			continue
		}
		if c == '"' || c == '\'' {
			inQuote = c
			continue
		}
		if c == '.' {
			if i+1 >= len(line) || line[i+1] == ' ' || line[i+1] == '\t' || line[i+1] == '\r' {
				return i, true
			}
		}
	}
	return 0, false
}

// AddRules 把規則加入程式（Index 依加入順序重排，保證穩定）。
func (p *Program) AddRules(rules []*Rule) error {
	for _, r := range rules {
		if r == nil {
			continue
		}
		cp := *r
		cp.Index = len(p.rules)
		rr := &cp
		if err := p.validateRule(rr); err != nil {
			return fmt.Errorf("rule %d (line %d): %w", rr.Index+1, rr.SrcLine, err)
		}
		if err := p.fixArity(rr.Head.Pred, len(rr.Head.Terms)); err != nil {
			return fmt.Errorf("rule %d (line %d): %w", rr.Index+1, rr.SrcLine, err)
		}
		for _, b := range rr.Body {
			if isBuiltin(b.Pred) {
				continue // 內建謂詞 arity 於 validateRule 檢查，不入 arity 表
			}
			if err := p.fixArity(b.Pred, len(b.Terms)); err != nil {
				return fmt.Errorf("rule %d (line %d): %w", rr.Index+1, rr.SrcLine, err)
			}
		}
		p.rules = append(p.rules, rr)
	}
	return nil
}

// MustParseRules 供內嵌常量規則使用（解析失敗即 panic——編譯期錯誤等同）。
func MustParseRules(src string) []*Rule {
	rules, err := ParseRules(src)
	if err != nil {
		panic("datalog: internal rules must parse: " + err.Error())
	}
	return rules
}

// ---------- 求值 ----------

// DerivedFact 是一條（可能由規則推導的）事實。
type DerivedFact struct {
	Pred    string
	Tuple   []string
	RuleIdx int // -1 = 外部注入事實（EDB）
}

// Eval 執行分層不動點求值，回傳「由規則推導」的全部事實（不含 EDB；按謂詞+項排序）。
func (p *Program) Eval() ([]DerivedFact, error) {
	strata, err := p.strataAssignment()
	if err != nil {
		return nil, err
	}
	edb := func(pred string, t []string) bool {
		set := p.facts[pred]
		if set == nil {
			return false
		}
		_, ok := set[tupleKey(t)]
		return ok
	}
	total := 0
	// 逐層（層號由小到大）對規則做 naive 不動點；層內迭代至無新事實。
	byStratum := map[int][]*Rule{}
	maxS := 0
	for _, r := range p.rules {
		s := strata[r.Head.Pred]
		byStratum[s] = append(byStratum[s], r)
		if s > maxS {
			maxS = s
		}
	}
	for s := 0; s <= maxS; s++ {
		rules := byStratum[s]
		if len(rules) == 0 {
			continue
		}
		for round := 0; round < p.bounds.MaxRounds; round++ {
			added := 0
			for _, r := range rules {
				n, err := p.fireRule(r, edb)
				if err != nil {
					return nil, err
				}
				added += n
				total += n
				if total > p.bounds.MaxDerivedFacts {
					return nil, fmt.Errorf("datalog: derived facts exceed limit %d (rule %d)", p.bounds.MaxDerivedFacts, r.Index+1)
				}
			}
			if added == 0 {
				break
			}
		}
	}
	return p.derivedSorted(), nil
}

// fireRule 對單條規則做一趟匹配-推導；回傳新增事實數。
// 正位置內建（neq）與負原子同樣作為「完整綁定後的過濾條件」（分層語意，見 filtersSatisfied）。
func (p *Program) fireRule(r *Rule, edb func(string, []string) bool) (int, error) {
	added := 0
	// 收集正位置非內建原子（用於變數綁定枚舉）。
	var pos []Atom
	for _, b := range r.Body {
		if b.Negated || isBuiltin(b.Pred) {
			continue
		}
		pos = append(pos, b.Atom)
	}
	if len(pos) == 0 {
		return 0, fmt.Errorf("rule must have at least one positive non-builtin body atom")
	}
	// 枚舉：以第一個正原子為種子（其謂詞的全部事實），DFS 綁定其餘正原子。
	var walk func(idx int, bind map[string]string)
	walk = func(idx int, bind map[string]string) {
		if idx == len(pos) {
			if !p.filtersSatisfied(r, bind, edb) {
				return
			}
			head, unbound := instantiate(r.Head, bind)
			if len(unbound) > 0 {
				return // 安全性已於驗證攔截；此處防禦性跳過
			}
			before := len(p.facts[r.Head.Pred])
			p.addGround(r.Head.Pred, head, r.Index)
			if len(p.facts[r.Head.Pred]) > before {
				added++
			}
			return
		}
		a := pos[idx]
		for _, entry := range p.sortedFacts(a.Pred) {
			bind2, ok := match(a, entry.tuple, bind)
			if !ok {
				continue
			}
			walk(idx+1, bind2)
		}
	}
	walk(0, map[string]string{})
	return added, nil
}

// filtersSatisfied 檢查全部過濾條件（負原子 + 正位置內建 neq）在綁定 bind 下是否同時成立。
func (p *Program) filtersSatisfied(r *Rule, bind map[string]string, edb func(string, []string) bool) bool {
	for _, b := range r.Body {
		if isBuiltin(b.Pred) {
			ok, err := evalBuiltin(b, bind)
			if err != nil || !ok {
				return false
			}
			continue
		}
		if !b.Negated {
			continue
		}
		t, unbound := instantiate(b.Atom, bind)
		if len(unbound) > 0 {
			return false
		}
		if edb(b.Pred, t) || p.hasFactLocked(b.Pred, t) {
			return false
		}
	}
	return true
}

func (p *Program) hasFactLocked(pred string, t []string) bool {
	set := p.facts[pred]
	if set == nil {
		return false
	}
	_, ok := set[tupleKey(t)]
	return ok
}

// sortedFacts 回傳該謂詞目前全部事實（排序確保枚舉順序穩定）。
func (p *Program) sortedFacts(pred string) []factEntry {
	set := p.facts[pred]
	out := make([]factEntry, 0, len(set))
	for _, e := range set {
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i].tuple, out[j].tuple
		for k := range a {
			if a[k] != b[k] {
				return a[k] < b[k]
			}
		}
		return false
	})
	return out
}

// derivedSorted 輸出「規則推導」的事實（排除 EDB），按 (pred, tuple) 排序。
func (p *Program) derivedSorted() []DerivedFact {
	var out []DerivedFact
	for pred, set := range p.facts {
		for _, e := range set {
			if e.ruleIdx < 0 {
				continue
			}
			out = append(out, DerivedFact{Pred: pred, Tuple: e.tuple, RuleIdx: e.ruleIdx})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Pred != out[j].Pred {
			return out[i].Pred < out[j].Pred
		}
		a, b := out[i].Tuple, out[j].Tuple
		for k := range a {
			if a[k] != b[k] {
				return a[k] < b[k]
			}
		}
		return out[i].RuleIdx < out[j].RuleIdx
	})
	return out
}

// ---------- 內建謂詞 ----------

func isBuiltin(pred string) bool { return pred == "neq" }

func evalBuiltin(b BodyAtom, bind map[string]string) (bool, error) {
	if b.Pred != "neq" || len(b.Terms) != 2 {
		return false, fmt.Errorf("unsupported builtin %q/%d", b.Pred, len(b.Terms))
	}
	v1, ok1 := resolve(b.Terms[0], bind)
	v2, ok2 := resolve(b.Terms[1], bind)
	if !ok1 || !ok2 {
		return false, fmt.Errorf("neq with unbound variable")
	}
	return v1 != v2, nil
}

// ---------- 綁定與匹配 ----------

func resolve(t Term, bind map[string]string) (string, bool) {
	if t.Kind == Const {
		return t.Val, true
	}
	v, ok := bind[t.Val]
	return v, ok
}

// match 嘗試把 atom 與 ground tuple 匹配，回傳擴充後的綁定（不改原 bind）。
func match(a Atom, tuple []string, bind map[string]string) (map[string]string, bool) {
	if len(a.Terms) != len(tuple) {
		return nil, false
	}
	out := bind
	copied := false
	for i, t := range a.Terms {
		if t.Kind == Const {
			if t.Val != tuple[i] {
				return nil, false
			}
			continue
		}
		if v, ok := out[t.Val]; ok {
			if v != tuple[i] {
				return nil, false
			}
			continue
		}
		if !copied {
			out = make(map[string]string, len(bind)+1)
			for k, v := range bind {
				out[k] = v
			}
			copied = true
		}
		out[t.Val] = tuple[i]
	}
	return out, true
}

// instantiate 以綁定實例化 atom；回傳 (常數元組, 未綁定變數列表)。
func instantiate(a Atom, bind map[string]string) ([]string, []string) {
	out := make([]string, len(a.Terms))
	var unbound []string
	for i, t := range a.Terms {
		if t.Kind == Const {
			out[i] = t.Val
			continue
		}
		if v, ok := bind[t.Val]; ok {
			out[i] = v
		} else {
			unbound = append(unbound, t.Val)
			out[i] = "?" + t.Val
		}
	}
	return out, unbound
}

// ---------- 驗證 ----------

func (p *Program) validateRule(r *Rule) error {
	if err := validPred(r.Head.Pred); err != nil {
		return err
	}
	if isBuiltin(r.Head.Pred) {
		return fmt.Errorf("head cannot be builtin predicate %q", r.Head.Pred)
	}
	var posPreds []Atom
	for _, b := range r.Body {
		if err := validPred(b.Pred); err != nil {
			return err
		}
		if isBuiltin(b.Pred) {
			if b.Negated {
				return fmt.Errorf("builtin %q cannot be negated", b.Pred)
			}
			if len(b.Terms) != 2 {
				return fmt.Errorf("builtin %q requires exactly 2 terms", b.Pred)
			}
			continue
		}
		if !b.Negated {
			posPreds = append(posPreds, b.Atom)
		}
	}
	if len(posPreds) == 0 {
		return fmt.Errorf("at least one positive non-builtin body atom is required")
	}
	// 安全性：頭部/過濾器的變數 ⊆ 正（非內建）原子綁定的變數。
	bound := map[string]bool{}
	for _, a := range posPreds {
		for _, t := range a.Terms {
			if t.Kind == Var {
				bound[t.Val] = true
			}
		}
	}
	for _, b := range r.Body {
		if !b.Negated && !isBuiltin(b.Pred) {
			continue
		}
		for _, t := range b.Terms {
			if t.Kind == Var && !bound[t.Val] {
				role := "negated atom"
				if isBuiltin(b.Pred) {
					role = "builtin filter"
				}
				return fmt.Errorf("variable %s in %s %q must be bound by a positive atom", t.Val, role, b.Pred)
			}
		}
	}
	for _, t := range r.Head.Terms {
		if t.Kind == Var && !bound[t.Val] {
			return fmt.Errorf("unsafe rule: head variable %s not bound by any positive body atom", t.Val)
		}
	}
	return nil
}

// strataAssignment 回傳謂詞 → 層號；負邊 q→p 要求 s(p) > s(q)。
// 迭代至不動點；超過 |edges|+1 輪仍有變動 → 負依賴成環（非分層程式）→ 報錯。
func (p *Program) strataAssignment() (map[string]int, error) {
	s := map[string]int{}
	// 初始：所有出現過的謂詞層 0。
	seen := map[string]bool{}
	for _, r := range p.rules {
		seen[r.Head.Pred] = true
		for _, b := range r.Body {
			if !isBuiltin(b.Pred) {
				seen[b.Pred] = true
			}
		}
	}
	for pred := range seen {
		s[pred] = 0
	}
	type edge struct {
		from, to string
		neg      bool
	}
	var edges []edge
	for _, r := range p.rules {
		for _, b := range r.Body {
			if isBuiltin(b.Pred) {
				continue
			}
			edges = append(edges, edge{b.Pred, r.Head.Pred, b.Negated})
		}
	}
	for i := 0; i <= len(edges); i++ {
		changed := false
		for _, e := range edges {
			req := s[e.from]
			if e.neg {
				req = s[e.from] + 1
			}
			if req > s[e.to] {
				s[e.to] = req
				changed = true
			}
		}
		if !changed {
			return s, nil
		}
	}
	return nil, fmt.Errorf("datalog: program is not stratifiable (negative dependency cycle)")
}

// ---------- 輔助 ----------

func (p *Program) fixArity(pred string, n int) error {
	if a, ok := p.arity[pred]; ok && a != n {
		return fmt.Errorf("predicate %s used with arity %d, want %d", pred, n, a)
	}
	p.arity[pred] = n
	return nil
}

func validPred(pred string) error {
	if pred == "" {
		return fmt.Errorf("empty predicate")
	}
	for i, r := range pred {
		if !(r == '_' || r == '.' || unicode.IsLetter(r) || (i > 0 && unicode.IsDigit(r))) {
			return fmt.Errorf("invalid predicate name %q", pred)
		}
	}
	return nil
}

func tupleKey(t []string) string { return strings.Join(t, "\x1f") }

// ---------- 規則文本解析（行式文法，與 chordlaw .dl 相容的子集） ----------

func stripComment(line string) string {
	if i := strings.IndexByte(line, '%'); i >= 0 {
		return line[:i]
	}
	return line
}

func parseRule(line string, lineNo int) (*Rule, error) {
	s := newScan(line, lineNo)
	head, err := s.atom()
	if err != nil {
		return nil, err
	}
	var body []BodyAtom
	if s.peekIs(":-") {
		s.accept(":-")
		for {
			neg := false
			if s.peekIs("!") {
				s.accept("!")
				neg = true
			}
			a, err := s.atom()
			if err != nil {
				return nil, err
			}
			body = append(body, BodyAtom{Atom: a, Negated: neg})
			if s.peekIs(",") {
				s.accept(",")
				continue
			}
			break
		}
	}
	if !s.accept(".") {
		return nil, s.errf("expected '.' at end of rule")
	}
	if !s.atEnd() {
		return nil, s.errf("unexpected trailing input")
	}
	return &Rule{Head: head, Body: body, SrcLine: lineNo}, nil
}

// scanner 是極簡詞法器。
type scanner struct {
	toks  []string
	pos   int
	line  int
	ident int // 匿名變數計數
}

func newScan(line string, lineNo int) *scanner {
	return &scanner{toks: tokenize(line), pos: 0, line: lineNo}
}

func tokenize(s string) []string {
	var toks []string
	i := 0
	for i < len(s) {
		c := s[i]
		switch {
		case c == ' ' || c == '\t' || c == '\r' || c == '\n':
			i++
		case c == '"' || c == '\'':
			q := c
			j := i + 1
			for j < len(s) && s[j] != q {
				j++
			}
			if j >= len(s) {
				toks = append(toks, s[i:])
				i = len(s)
			} else {
				toks = append(toks, s[i:j+1])
				i = j + 1
			}
		case isSymChar(c):
			// ":-" 是唯一雙字元符號
			if c == ':' && i+1 < len(s) && s[i+1] == '-' {
				toks = append(toks, ":-")
				i += 2
			} else {
				toks = append(toks, string(c))
				i++
			}
		default:
			j := i
			for j < len(s) {
				c := s[j]
				if isSymChar(c) && !(c == '.' && j+1 < len(s) && isIdentRune(s[j+1])) {
					break // 符號；例外：'x.y' 形式的謂詞命名（'.' 後緊接識別字元）
				}
				if c == ' ' || c == '\t' || c == '\r' || c == '\n' {
					break
				}
				j++
			}
			toks = append(toks, s[i:j])
			i = j
		}
	}
	return toks
}

func isSymChar(c byte) bool {
	switch c {
	case '(', ')', ',', '.', '!', ':':
		return true
	}
	return false
}

// isIdentRune：識別字元（字母/數字/底線；tokenize 用 byte 級判斷足夠——
// 非 ASCII 位元組一律視為識別字元的一部分，與 validPred 的 unicode 檢查相容）。
func isIdentRune(c byte) bool {
	return c == '_' || c >= 0x80 || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}

func (s *scanner) peekIs(tok string) bool {
	return s.pos < len(s.toks) && s.toks[s.pos] == tok
}

func (s *scanner) accept(tok string) bool {
	if s.peekIs(tok) {
		s.pos++
		return true
	}
	return false
}

func (s *scanner) atEnd() bool { return s.pos >= len(s.toks) }

func (s *scanner) errf(format string, args ...any) error {
	return fmt.Errorf("datalog line %d: %s", s.line, fmt.Sprintf(format, args...))
}

func (s *scanner) next() (string, error) {
	if s.atEnd() {
		return "", s.errf("unexpected end of rule")
	}
	t := s.toks[s.pos]
	s.pos++
	return t, nil
}

func (s *scanner) atom() (Atom, error) {
	name, err := s.next()
	if err != nil {
		return Atom{}, err
	}
	if len(name) == 0 || isSymChar(name[0]) || name[0] == '"' || name[0] == '\'' {
		return Atom{}, s.errf("expected predicate name, got %q", name)
	}
	if !s.accept("(") {
		// 零參數謂詞（如 diag.present、epoch.has_work）——無括號形式。
		return Atom{Pred: name}, nil
	}
	var terms []Term
	for {
		tok, err := s.next()
		if err != nil {
			return Atom{}, err
		}
		terms = append(terms, s.term(tok))
		if s.accept(",") {
			continue
		}
		break
	}
	if !s.accept(")") {
		return Atom{}, s.errf("expected ')' to close atom %q", name)
	}
	return Atom{Pred: name, Terms: terms}, nil
}

func (s *scanner) term(tok string) Term {
	if len(tok) >= 2 && (tok[0] == '"' || tok[0] == '\'') {
		return Term{Kind: Const, Val: tok[1 : len(tok)-1]}
	}
	if tok == "_" {
		s.ident++
		return Term{Kind: Var, Val: "_Anon" + strconv.Itoa(s.ident)}
	}
	if tok[0] >= 'A' && tok[0] <= 'Z' {
		return Term{Kind: Var, Val: tok}
	}
	return Term{Kind: Const, Val: tok}
}
