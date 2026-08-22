package kb

import "sort"

// 依賴項圖（代理用）：原子間以 Refs 邊構成有向圖。
//
//   - Deps(id)       → 本原子依賴的原子（前驅/前置知識）
//   - Dependents(id) → 引用本原子的原子（後繼/誰需要我）
//
// 上下文展開（ExpandContext）在「依賴 ∪ 被依賴」的無向化圖上做 BFS，
// 因此查 E0382 會帶出引用它的 OWN-* 規則，再帶出規則引用的章節——
// 這正是代理需要的「答案 + 背後的規則 + 出處」閉包。
//
// 注意：相關概念彼此引用（如 BRW-01 ↔ BRW-03）會形成環，這是正常的
// 「相關性環」，BFS 以 visited 集合去重、有界深度，不受影響；
// Cycles() 僅作診斷用，回報強連通分量。
type graph struct {
	deps map[string][]string // atomID → 依賴的 atomID
	rev  map[string][]string // atomID → 引用它的 atomID
}

func newGraph(atoms []*Atom) *graph {
	g := &graph{deps: map[string][]string{}, rev: map[string][]string{}}
	for _, a := range atoms {
		g.deps[a.ID] = append([]string(nil), a.Refs...)
		for _, r := range a.Refs {
			g.rev[r] = append(g.rev[r], a.ID)
		}
	}
	for id := range g.deps {
		sort.Strings(g.deps[id])
	}
	for id := range g.rev {
		g.rev[id] = dedupe(g.rev[id])
		sort.Strings(g.rev[id])
	}
	return g
}

// Deps 回傳指定原子依賴的原子 ID（排序，決定論）。
func (g *graph) Deps(id string) []string { return g.deps[id] }

// Dependents 回傳引用指定原子的原子 ID（排序，決定論）。
func (g *graph) Dependents(id string) []string { return g.rev[id] }

// ExpandContext 由根原子出發，在依賴 ∪ 被依賴圖上做 BFS 至 depth 層，
// 回傳去重的原子 ID 序列（決定論）：根依傳入順序在前，其後逐層、層內依 ID
// 排序——因此根的順序（如檢索相關性順序）會被保留。
func (g *graph) ExpandContext(roots []string, depth int) []string {
	if depth < 0 {
		depth = 0
	}
	type node struct {
		id    string
		depth int
	}
	seen := map[string]bool{}
	var order []node
	queue := make([]string, 0, len(roots))
	for _, r := range roots {
		if !seen[r] {
			seen[r] = true
			queue = append(queue, r)
			order = append(order, node{r, 0})
		}
	}
	head := 0
	for head < len(queue) {
		cur := queue[head]
		head++
		var d int
		for _, n := range order {
			if n.id == cur {
				d = n.depth
				break
			}
		}
		if d >= depth {
			continue
		}
		neigh := append(append([]string{}, g.deps[cur]...), g.rev[cur]...)
		sort.Strings(neigh)
		for _, n := range neigh {
			if !seen[n] {
				seen[n] = true
				queue = append(queue, n)
				order = append(order, node{n, d + 1})
			}
		}
	}
	out := make([]string, len(order))
	for i, n := range order {
		out[i] = n.id
	}
	return out
}

// Cycles 回傳依賴圖中的強連通分量（大小 > 1），僅作診斷。
// 相關概念互引（如 BRW-01 ↔ BRW-03）會自然成環——這是「相關性環」，
// 非錯誤；呼叫方應知曉並據此決定是否以「去環」策略處理。
func (g *graph) Cycles() [][]string {
	// Kosaraju（簡單、決定論）。
	visited := map[string]bool{}
	var finish []string
	var dfs func(u string)
	dfs = func(u string) {
		visited[u] = true
		for _, v := range g.deps[u] {
			if !visited[v] {
				dfs(v)
			}
		}
		finish = append(finish, u)
	}
	// 對所有節點（以排序鍵保證決定論）跑第一趟。
	keys := make([]string, 0, len(g.deps))
	for k := range g.deps {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if !visited[k] {
			dfs(k)
		}
	}
	// 第二趟：在反圖上依 finish 逆序找 SCC。
	visited = map[string]bool{}
	var scc [][]string
	var collect func(u string, comp *[]string)
	collect = func(u string, comp *[]string) {
		visited[u] = true
		*comp = append(*comp, u)
		for _, v := range g.rev[u] {
			if !visited[v] {
				collect(v, comp)
			}
		}
	}
	for i := len(finish) - 1; i >= 0; i-- {
		u := finish[i]
		if visited[u] {
			continue
		}
		var comp []string
		collect(u, &comp)
		sort.Strings(comp)
		if len(comp) > 1 {
			scc = append(scc, comp)
		}
	}
	sort.Slice(scc, func(i, j int) bool { return scc[i][0] < scc[j][0] })
	return scc
}
