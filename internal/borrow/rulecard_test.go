package borrow

import (
	"strings"
	"testing"
)

func TestRuleCardComplete(t *testing.T) {
	if len(RuleCard) != 10 {
		t.Fatalf("規則卡 = %d 條, want 10 (E01–E10)", len(RuleCard))
	}
	seen := map[string]bool{}
	for i, e := range RuleCard {
		if e.ID == "" || e.Name == "" || e.Geometry == "" {
			t.Errorf("條目 %d 欄位缺失: %+v", i, e)
		}
		if e.Law != 1 && e.Law != 2 {
			t.Errorf("[%s] 法則編號非法: %d", e.ID, e.Law)
		}
		if len(e.RustcCodes) == 0 {
			t.Errorf("[%s] 無 rustc 映射", e.ID)
		}
		if len(e.Fixes) < 2 {
			t.Errorf("[%s] 修法菜單少於 2 項（失去封閉選擇題價值）", e.ID)
		}
		if seen[e.ID] {
			t.Errorf("[%s] 重複", e.ID)
		}
		seen[e.ID] = true
	}
}

func TestRuleCardRustcMappingConsistency(t *testing.T) {
	// 規則卡中出現的每個 rustc 碼都必須被 IsBorrowCode 認得
	for _, e := range RuleCard {
		for _, c := range e.RustcCodes {
			if !IsBorrowCode(c) {
				t.Errorf("[%s] 映射碼 %s 不在 BorrowRustcCodes", e.ID, c)
			}
		}
	}
	// 反向：BorrowRustcCodes 中的碼要能查到規則或模板（無孤兒碼）
	for c := range BorrowRustcCodes {
		if len(ByRustcCode(c)) == 0 && TemplateFor(c) == nil {
			t.Errorf("孤兒碼 %s: 既無規則條目也無模板", c)
		}
	}
}

func TestByRustcCode(t *testing.T) {
	es := ByRustcCode("E0502")
	if len(es) < 2 { // E01 與 E09 都映射 E0502
		t.Fatalf("E0502 應至少映射 2 條規則, got %d", len(es))
	}
	if len(ByRustcCode("E9999")) != 0 {
		t.Fatal("未知碼應回空")
	}
}

func TestByID(t *testing.T) {
	if e := ByID("E01"); e == nil || e.Name != "紅弧交越" {
		t.Fatalf("ByID(E01) = %+v", e)
	}
	if ByID("E99") != nil {
		t.Fatal("ByID 未知碼應回 nil")
	}
}

func TestRenderCard(t *testing.T) {
	out := RenderCard(RuleCard)
	for _, want := range []string{"法則①", "法則②", "[E01]", "[E10]", "E0499", "修法:"} {
		if !strings.Contains(out, want) {
			t.Errorf("規則卡渲染缺 %q", want)
		}
	}
	// 子集渲染
	sub := RenderCard(ByRustcCode("E0106"))
	if !strings.Contains(sub, "[E10]") || strings.Contains(sub, "[E01]") {
		t.Error("子集渲染錯誤")
	}
}

func TestIsBorrowCode(t *testing.T) {
	for _, c := range []string{"E0499", "E0502", "E0382", "E0597", "E0106"} {
		if !IsBorrowCode(c) {
			t.Errorf("%s 應為 borrow 類", c)
		}
	}
	for _, c := range []string{"E0425", "E0308", ""} { // 名稱解析/型別錯誤非 borrow 類
		if IsBorrowCode(c) {
			t.Errorf("%s 不應為 borrow 類", c)
		}
	}
}

func TestTemplateFor(t *testing.T) {
	for _, c := range []string{"E0502", "E0499", "E0506", "E0503", "E0382", "E0505", "E0597", "E0106"} {
		if TemplateFor(c) == nil {
			t.Errorf("缺 %s 模板", c)
		}
	}
	// 別名歸併
	if tp := TemplateFor("E0515"); tp == nil || tp.RustcCode != "E0106" {
		t.Error("E0515 應歸併到 E0106 模板")
	}
	if tp := TemplateFor("E0507"); tp == nil || tp.RustcCode != "E0505" {
		t.Error("E0507 應歸併到 E0505 模板")
	}
	if TemplateFor("E0425") != nil {
		t.Error("非 borrow 碼不應有模板")
	}
}
