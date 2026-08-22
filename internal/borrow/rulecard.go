package borrow

import (
	"fmt"
	"strings"
)

// ============================================================
// 幾何規則卡：把 rustc 借用規則壓縮為兩條幾何法則 + 封閉修法菜單。
// 目的：把代理的「開放式除錯」變成「封閉式選擇題」。
// 內容 100% 靜態——即使 L5 引擎(python)不可用, 規則卡仍可用。
// 來源：ChordLaw rules.dl E01–E10 與 26 例 rustc oracle 對照。
// ============================================================

// 兩條幾何法則（印於每張規則卡頂部）。
const (
	Law1 = "法則① 紅弧孤立: mut 借用的弧跨內不得含其他借用的端點、不得與其他弧交越（排他性的幾何形式）"
	Law2 = "法則② 弧在圓內: 借用弧的端點必須落在被借者的作用域圓內; 回傳的引用, 其被借者必須活得比呼叫者久"
)

// RuleEntry 是一條幾何規則。
type RuleEntry struct {
	ID         string   // E01…
	Name       string   // 紅弧交越
	Law        int      // 1 | 2
	RustcCodes []string // 對應 rustc 錯誤碼
	Geometry   string   // 幾何語義一句話
	Fixes      []string // 修法菜單（封閉選擇題）
}

// RuleCard 是完整的十條規則（順序即 ID 序）。
var RuleCard = []RuleEntry{
	{
		ID: "E01", Name: "紅弧交越", Law: 1,
		RustcCodes: []string{"E0499", "E0502"},
		Geometry:   "路徑衝突的兩借用區間重疊, 且至少一者為 mut",
		Fixes: []string{
			"縮短先前借用的區間: 把它的最後使用移到新借用誕生之前（NLL 順序化）",
			"延後新借用: 移到先前借用死亡之後",
			"拆分路徑（split borrow）: 若觸及不同字段, 借 x.f 與 x.g 而非整體 x",
		},
	},
	{
		ID: "E02", Name: "借用期間寫入被借者", Law: 1,
		RustcCodes: []string{"E0506"},
		Geometry:   "寫入點落在活躍借用的弧跨內（寫=對弧的入侵）",
		Fixes: []string{
			"把寫入移到借用區間結束之後（讓引用先用完）",
			"把借用的最後使用提前到寫入之前",
			"若寫入不同字段: 拆分路徑借用",
		},
	},
	{
		ID: "E03", Name: "mut 借用期間讀取被借者", Law: 1,
		RustcCodes: []string{"E0503"},
		Geometry:   "讀取點落在 mut 弧跨內（紅弧內不得有他人端點）",
		Fixes: []string{
			"經由該 mut 引用讀取（*r），而非直接讀被借者",
			"縮短 mut 借用區間, 讀取移到其後",
		},
	},
	{
		ID: "E04", Name: "使用點逸出作用域", Law: 2,
		RustcCodes: []string{"E0597"},
		Geometry:   "使用點落在被借者的作用域圓之外（弧端點出圓）",
		Fixes: []string{
			"提升被借者宣告位置: 移到外層作用域, 讓圓包住所有使用點",
			"把使用移進被借者的作用域內",
		},
	},
	{
		ID: "E05", Name: "drop 後使用", Law: 2,
		RustcCodes: []string{"E0382"},
		Geometry:   "讀取點在顯式 drop 點之後（引用指向已釋放的點）",
		Fixes: []string{
			"把 drop 延後到所有使用之後",
			"重新初始化: drop 後對該 place 整體寫入即可再用",
		},
	},
	{
		ID: "E06", Name: "move 後使用", Law: 2,
		RustcCodes: []string{"E0382", "E0594"},
		Geometry:   "使用點在 move 點之後且無重新初始化（含: 整體 move 後觸及子字段）",
		Fixes: []string{
			"改為借用而非 move（&x / &mut x）",
			"clone 後再 move",
			"move 後、使用前重新初始化（整體寫入豁免）",
		},
	},
	{
		ID: "E07", Name: "借用期間 move 被借者", Law: 1,
		RustcCodes: []string{"E0505"},
		Geometry:   "move 點落在活躍借用的弧跨內（move=把被借者抽走, 弧失去錨點）",
		Fixes: []string{
			"把 move 延後到借用區間結束之後",
			"把借用的最後使用提前到 move 之前",
		},
	},
	{
		ID: "E08", Name: "借用期間 drop 被借者", Law: 1,
		RustcCodes: []string{"E0505"},
		Geometry:   "顯式 drop 點落在活躍借用的弧跨內",
		Fixes: []string{
			"把 drop 延後到借用區間結束之後",
			"讓借用自然死亡（NLL）而非顯式 drop 被借者",
		},
	},
	{
		ID: "E09", Name: "別名層衝突", Law: 1,
		RustcCodes: []string{"E0499", "E0502"},
		Geometry:   "引用 t 的使用點落在「t 自身被借用」的弧跨內（弧上有弧, 內層排他被破壞）",
		Fixes: []string{
			"經由最外層引用使用（用 b 而非 a, 若 b=&a）",
			"縮短對 t 的借用區間, t 的直接使用移到其後",
		},
	},
	{
		ID: "E10", Name: "引用逸出", Law: 2,
		RustcCodes: []string{"E0106", "E0515", "E0716"},
		Geometry:   "回傳引用的被借者是局部變數（弧穿出 fn 的圓）",
		Fixes: []string{
			"改返回擁有值（move 出去）而非引用",
			"被借者由呼叫者提供（參數傳入 = 弧錨在呼叫者圓內）",
			"延長被借者生命週期（'static / 提升所有權層級）",
		},
	},
}

// BorrowRustcCodes 是「屬 borrow/生命週期類」的 rustc 錯誤碼全集
// （L4 掛鉤用: 命中即觸發 L5 解釋）。
var BorrowRustcCodes = map[string]bool{
	"E0106": true, "E0382": true, "E0499": true, "E0502": true,
	"E0503": true, "E0505": true, "E0506": true, "E0507": true,
	"E0515": true, "E0594": true, "E0597": true, "E0716": true,
}

// IsBorrowCode 判斷 rustc 錯誤碼是否屬 borrow 類。
func IsBorrowCode(code string) bool { return BorrowRustcCodes[code] }

// ByRustcCode 回傳映射到指定 rustc 錯誤碼的規則條目。
func ByRustcCode(code string) []RuleEntry {
	var out []RuleEntry
	for _, e := range RuleCard {
		for _, c := range e.RustcCodes {
			if c == code {
				out = append(out, e)
				break
			}
		}
	}
	return out
}

// ByID 回傳指定 ChordLaw 錯誤碼的規則條目（無則 nil）。
func ByID(id string) *RuleEntry {
	for i := range RuleCard {
		if RuleCard[i].ID == id {
			return &RuleCard[i]
		}
	}
	return nil
}

// RenderCard 渲染規則卡（全部或子集）為代理可讀文本。
func RenderCard(entries []RuleEntry) string {
	var b strings.Builder
	b.WriteString("Rust 借用規則幾何卡 (ChordLaw E01–E10 ↔ rustc; 兩條法則覆蓋一切)\n")
	b.WriteString("  " + Law1 + "\n")
	b.WriteString("  " + Law2 + "\n\n")
	for _, e := range entries {
		fmt.Fprintf(&b, "[%s] %s  (法則%s; ~ rustc %s)\n",
			e.ID, e.Name, lawNum(e.Law), strings.Join(e.RustcCodes, "/"))
		fmt.Fprintf(&b, "  幾何: %s\n  修法:\n", e.Geometry)
		for i, f := range e.Fixes {
			fmt.Fprintf(&b, "    %d. %s\n", i+1, f)
		}
	}
	return b.String()
}

func lawNum(n int) string {
	if n == 1 {
		return "①"
	}
	return "②"
}
