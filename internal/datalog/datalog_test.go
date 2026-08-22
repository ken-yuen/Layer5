package datalog

import (
	"strings"
	"testing"
)

func findFacts(ds []DerivedFact, pred string) [][]string {
	var out [][]string
	for _, d := range ds {
		if d.Pred == pred {
			out = append(out, d.Tuple)
		}
	}
	return out
}

// TestTransitiveClosure：正遞迴（chordlaw reach 同型）。
func TestTransitiveClosure(t *testing.T) {
	p := NewProgram()
	rs, err := ParseRules("reach(A,B) :- edge(A,B).\nreach(A,C) :- reach(A,B), edge(B,C).")
	if err != nil {
		t.Fatal(err)
	}
	if err := p.AddRules(rs); err != nil {
		t.Fatal(err)
	}
	for _, e := range [][2]string{{"a", "b"}, {"b", "c"}, {"c", "d"}} {
		if err := p.AddFact("edge", e[0], e[1]); err != nil {
			t.Fatal(err)
		}
	}
	ds, err := p.Eval()
	if err != nil {
		t.Fatalf("eval: %v", err)
	}
	got := findFacts(ds, "reach")
	if len(got) != 6 { // a→b,a→c,a→d,b→c,b→d,c→d
		t.Fatalf("expected 6 reach facts, got %d: %v", len(got), got)
	}
}

// TestStratifiedNegation：分層否定 + neq。
func TestStratifiedNegation(t *testing.T) {
	p := NewProgram()
	rs, err := ParseRules(`
dangerous(X) :- driver(X), !licensed(X).
conflict(A,B) :- driver(A), driver(B), neq(A,B).
`)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.AddRules(rs); err != nil {
		t.Fatal(err)
	}
	for _, d := range []string{"alice", "bob"} {
		_ = p.AddFact("driver", d)
	}
	_ = p.AddFact("licensed", "alice")
	ds, err := p.Eval()
	if err != nil {
		t.Fatalf("eval: %v", err)
	}
	dan := findFacts(ds, "dangerous")
	if len(dan) != 1 || dan[0][0] != "bob" {
		t.Fatalf("expected only bob dangerous, got %v", dan)
	}
	con := findFacts(ds, "conflict")
	if len(con) != 2 { // alice-bob 與 bob-alice（有序對）
		t.Fatalf("expected 2 conflict pairs, got %v", con)
	}
}

// TestNegativeCycleRejected：非分層程式必須顯式報錯。
func TestNegativeCycleRejected(t *testing.T) {
	p := NewProgram()
	rs, err := ParseRules(`
a(X) :- b(X), !c(X).
b(X) :- a(X).
c(X) :- a(X), !b(X).
`)
	if err != nil {
		t.Fatal(err)
	}
	_ = p.AddRules(rs)
	if _, err := p.Eval(); err == nil || !strings.Contains(err.Error(), "stratifiable") {
		t.Fatalf("expected stratification error, got %v", err)
	}
}

// TestUnsafeRuleRejected：頭部變數未綁定 → 拒絕。
func TestUnsafeRuleRejected(t *testing.T) {
	p := NewProgram()
	rs, _ := ParseRules(`bad(X, Y) :- uses(X).`)
	if err := p.AddRules(rs); err == nil || !strings.Contains(err.Error(), "unsafe") {
		t.Fatalf("expected unsafe rule error, got %v", err)
	}
}

// TestNegationUnboundRejected：否定內未綁定變數 → 拒絕。
func TestNegationUnboundRejected(t *testing.T) {
	p := NewProgram()
	rs, _ := ParseRules(`bad(X) :- uses(X), !other(Y).`)
	if err := p.AddRules(rs); err == nil || !strings.Contains(err.Error(), "negated atom") {
		t.Fatalf("expected negation-bound error, got %v", err)
	}
}

// TestQuotedConstantsWithSpaces：reason 文案含空格/逗點必須完整保留。
func TestQuotedConstantsWithSpaces(t *testing.T) {
	p := NewProgram()
	rs, err := ParseRules(`
violation("fake_test_claim", "critical", "agent claimed tests passed without fresh evidence", "") :- claim("tests_passed"), !evidence("test").
`)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.AddRules(rs); err != nil {
		t.Fatal(err)
	}
	_ = p.AddFact("claim", "tests_passed")
	ds, err := p.Eval()
	if err != nil {
		t.Fatal(err)
	}
	v := findFacts(ds, "violation")
	if len(v) != 1 || v[0][2] != "agent claimed tests passed without fresh evidence" {
		t.Fatalf("quoted reason mismatch: %v", v)
	}
}

// TestDeterminism：兩次求值輸出完全一致（裁判可重放對賬）。
func TestDeterminism(t *testing.T) {
	src := `
reach(A,B) :- edge(A,B).
reach(A,C) :- reach(A,B), edge(B,C).
flag(X) :- reach(a, X), !skip(X).
`
	build := func() *Program {
		p := NewProgram()
		rs, _ := ParseRules(src)
		_ = p.AddRules(rs)
		_ = p.AddFact("edge", "a", "b")
		_ = p.AddFact("edge", "b", "c")
		_ = p.AddFact("skip", "c")
		return p
	}
	d1, err := build().Eval()
	if err != nil {
		t.Fatal(err)
	}
	d2, err := build().Eval()
	if err != nil {
		t.Fatal(err)
	}
	if len(d1) != len(d2) {
		t.Fatalf("nondeterministic lengths: %d vs %d", len(d1), len(d2))
	}
	for i := range d1 {
		if d1[i].Pred != d2[i].Pred || tupleKey(d1[i].Tuple) != tupleKey(d2[i].Tuple) {
			t.Fatalf("nondeterministic at %d: %v vs %v", i, d1[i], d2[i])
		}
	}
}

// TestParseErrors：常見語法錯誤顯式報錯。
func TestParseErrors(t *testing.T) {
	for _, bad := range []string{
		"head(X",                       // 缺右括
		"head(X) :- .",                 // 空 body
		"head(X). trailing",            // 尾隨輸入
		`head("unterminated) :- a(X).`, // 引號未閉
	} {
		if _, err := ParseRules(bad); err == nil {
			t.Fatalf("expected error for %q", bad)
		}
	}
}

// TestArityMismatch：同謂詞 arity 漂移 → 報錯（防規則集內部不一致）。
func TestArityMismatch(t *testing.T) {
	p := NewProgram()
	rs, _ := ParseRules("a(X) :- b(X, Y).\na(X) :- b(X, Y, Z).")
	if err := p.AddRules(rs); err == nil || !strings.Contains(err.Error(), "arity") {
		t.Fatalf("expected arity error, got %v", err)
	}
}

// TestBuiltinNegatedRejected：neq 不得否定；正位置 neq 是合法過濾器。
func TestBuiltinNegatedRejected(t *testing.T) {
	p := NewProgram()
	rs, _ := ParseRules("a(X) :- b(X), !neq(X, Y).")
	if err := p.AddRules(rs); err == nil || !strings.Contains(err.Error(), "cannot be negated") {
		t.Fatalf("expected negated-neq rejection, got %v", err)
	}
}

// TestDottedPredicateNames：謂詞可含 '.'（claim.kind 風格）；句點規則結尾不受影響。
func TestDottedPredicateNames(t *testing.T) {
	p := NewProgram()
	rs, err := ParseRules(`
violation("x", "critical", "msg", "", "") :- claim.kind("tests_passed"), !evidence.latest("test").
ok(K) :- claim.kind(K), neq(K, "none").
`)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.AddRules(rs); err != nil {
		t.Fatal(err)
	}
	_ = p.AddFact("claim.kind", "tests_passed")
	ds, err := p.Eval()
	if err != nil {
		t.Fatal(err)
	}
	if len(findFacts(ds, "violation")) != 1 {
		t.Fatalf("expected 1 violation, got %v", ds)
	}
	if len(findFacts(ds, "ok")) != 1 {
		t.Fatalf("expected 1 ok, got %v", ds)
	}
}
