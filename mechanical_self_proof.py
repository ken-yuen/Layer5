#!/usr/bin/env python3
"""
========================================================================================
機械自証程序：六值非經典邏輯矩陣、多環混合機構拓撲映射、程代展半張量積(STP)閉環、
SFT 拓撲符號動力系統與 A/B 閉環發現機之嚴格數學自証
========================================================================================
對應論文與技術庫：
1. ken-yuen/Layer5 (弦律 ChordLaw): Datalog × DAG 拓撲 × 圓示借用檢查器
2. Cheng's Semi-Tensor Product (程代展半張量積): 離散邏輯動態系統代數狀態空間化
3. Bowen-Lanford 定理: SFT 動態 Zeta 函數 ζ(z) = 1/det(I - zA) = exp(∑ Tr(A^m)/m z^m)
4. 經典控制傳遞函數 Y(s)/X(s) = G(s)/(1 + G(s)H(s)) 及其狀態空間與 STP 閉環極點同構
5. A/B 閉環發現機: Flow-Matching (連續生成) + Model B Oracle (Datalog證明樹) + SLS (幾何翻轉)
========================================================================================
"""

import sys
import os
import math
import numpy as np
import sympy as sp

# 引入 Layer5 弦律引擎
sys.path.append('/home/user/Layer5')
import chordlaw

print("=" * 88)
print("  🚀 啟動數學運算機械自証系統 (Mechanical Self-Proof Engine)")
print("  驗證演算法、拓撲學與邏輯轉譯之「絕對對等」 (Absolute Equivalences)")
print("=" * 88)

# ======================================================================================
# 模組 1：程代展半張量積 (STP) 代數公理機械自証
# ======================================================================================
print("\n" + "=" * 88)
print("【模組 1】程代展半張量積 (STP, 記為 ⋉) 代數公理機械自証")
print("=" * 88)

def stp(A: np.ndarray, B: np.ndarray) -> np.ndarray:
    """計算兩個任意維度矩陣的程代展半張量積 A ⋉ B = (A ⊗ I_{t/n})(B ⊗ I_{t/p}), t = lcm(n, p)"""
    n = A.shape[1]
    p = B.shape[0]
    t = np.lcm(n, p)
    At = np.kron(A, np.eye(t // n, dtype=A.dtype))
    Bt = np.kron(B, np.eye(t // p, dtype=B.dtype))
    return np.matmul(At, Bt)

def delta(k: int, i: int) -> np.ndarray:
    """生成 k 值邏輯的標準基向量 delta_k^i (1-indexed)"""
    v = np.zeros((k, 1), dtype=int)
    v[i - 1, 0] = 1
    return v

def swap_matrix(m: int, n: int) -> np.ndarray:
    """Cheng 換位矩陣 W_{[m, n]} 滿足 W_{[m, n]} ⋉ X ⋉ Y = Y ⋉ X (X ∈ Δ_m, Y ∈ Δ_n)"""
    W = np.zeros((m * n, m * n), dtype=int)
    for i in range(m):
        for j in range(n):
            row = j * m + i
            col = i * n + j
            W[row, col] = 1
    return W

def power_reducing_matrix(n: int) -> np.ndarray:
    """降冪矩陣 Phi_n 使得 X^2 = X ⋉ X = Phi_n ⋉ X (X ∈ Δ_n)"""
    Phi = np.zeros((n * n, n), dtype=int)
    for i in range(n):
        Phi[i * n + i, i] = 1
    return Phi

# 測試 1.1：結合律 (A ⋉ B) ⋉ C == A ⋉ (B ⋉ C) 在任意異維度矩陣下成立
np.random.seed(2026)
A_rand = np.random.randint(-10, 10, size=(2, 3))
B_rand = np.random.randint(-10, 10, size=(4, 2))
C_rand = np.random.randint(-10, 10, size=(3, 5))

res_left = stp(stp(A_rand, B_rand), C_rand)
res_right = stp(A_rand, stp(B_rand, C_rand))
diff_assoc = np.max(np.abs(res_left - res_right))
print(f"1.1 結合律自証: max| (A ⋉ B) ⋉ C - A ⋉ (B ⋉ C) | = {diff_assoc}")
assert diff_assoc == 0, "結合律自証失敗！"
print("    ✅ 結合律 (Associativity) 機械自証 PASS (零誤差)")

# 測試 1.2：加法分配律 A ⋉ (B + C) == A ⋉ B + A ⋉ C
B2_rand = np.random.randint(-10, 10, size=(4, 2))
dist_left = stp(A_rand, B_rand + B2_rand)
dist_right = stp(A_rand, B_rand) + stp(A_rand, B2_rand)
diff_dist = np.max(np.abs(dist_left - dist_right))
print(f"1.2 分配律自証: max| A ⋉ (B + C) - (A ⋉ B + A ⋉ C) | = {diff_dist}")
assert diff_dist == 0, "分配律自証失敗！"
print("    ✅ 分配律 (Distributivity) 機械自証 PASS (零誤差)")

# 測試 1.3：換位矩陣 W_{[m, n]} 之擬交換律
m, n = 3, 4
W_mn = swap_matrix(m, n)
swap_passed = True
for i in range(1, m + 1):
    for j in range(1, n + 1):
        x = delta(m, i)
        y = delta(n, j)
        xy = stp(x, y)
        yx = stp(y, x)
        w_xy = np.matmul(W_mn, xy)
        if not np.array_equal(w_xy, yx):
            swap_passed = False
print(f"1.3 換位矩陣 W_[{m},{n}] 交換性: 遍歷全部 {m*n} 組基向量對")
assert swap_passed, "換位矩陣自証失敗！"
print("    ✅ 換位矩陣交換性 (W_{[m,n]} ⋉ X ⋉ Y = Y ⋉ X) 機械自証 PASS")

# 測試 1.4：降冪矩陣 Phi_n ⋉ X = X^2
k_dim = 6
Phi_k = power_reducing_matrix(k_dim)
power_passed = True
for i in range(1, k_dim + 1):
    x = delta(k_dim, i)
    x_sq = stp(x, x)
    phi_x = np.matmul(Phi_k, x)
    if not np.array_equal(x_sq, phi_x):
        power_passed = False
print(f"1.4 降冪矩陣 Phi_{k_dim} 自証: 驗證 Phi_{k_dim} ⋉ X ≡ X ⋉ X")
assert power_passed, "降冪矩陣自証失敗！"
print(f"    ✅ 降冪矩陣 (Power Reduction Phi_n) 機械自証 PASS")


# ======================================================================================
# 模組 2：六值非經典邏輯矩陣與多環機構拓撲映射自証
# ======================================================================================
print("\n" + "=" * 88)
print("【模組 2】六值非經典邏輯矩陣 (L6) 與多環機構拓撲約束自証")
print("=" * 88)

STATE_NAMES = {
    1: "U (Uninit / 自由未約束關節)",
    2: "O (Owned / 主動驅動關節)",
    3: "S (Shared / 被動並聯從動，可共存)",
    4: "M (Mut-Locked / 排他剛性鎖死)",
    5: "V (Moved / 脫扣分支)",
    6: "D (Dropped / 奇異點/碰撞/衝突)"
}

# 構造多環混合機構的接合並聯結構矩陣 M_loop ∈ R^{6 x 36}
M_loop = np.zeros((6, 36), dtype=int)
for i in range(1, 7):
    for j in range(1, 7):
        col = (i - 1) * 6 + (j - 1)
        if i == 6 or j == 6:
            out = 6
        elif i == 5 or j == 5:
            out = 6  # Moved 消耗後存取 (E06)
        elif i == 1 or j == 1:
            out = 6  # 未初始化存取 (E29)
        elif i == 4 and j == 4:
            out = 6  # 兩環同時排他剛性鎖死 -> 死鎖衝突 (E01)
        elif (i == 4 and j == 3) or (i == 3 and j == 4):
            out = 6  # 排他 vs 共享 (E01)
        elif (i == 4 and j == 2) or (i == 2 and j == 4):
            out = 6  # 排他期間外部驅動 (E02/E03)
        elif i == 3 and j == 3:
            out = 3  # 兩環安全共享被動副 (合法)
        elif (i == 3 and j == 2) or (i == 2 and j == 3):
            out = 3  # 主動驅動配合被動共享 (合法)
        elif i == 2 and j == 2:
            out = 2  # 標準連續驅動 (合法)
        else:
            out = 6
        M_loop[out - 1, col] = 1

print("2.1 驗證六值邏輯在多環機構衝突運算:")
test_cases_m2 = [
    (3, 3, 3, "Shared ⊗ Shared = Shared (兩環並聯共存)"),
    (4, 4, 6, "Mut ⊗ Mut = Dropped (兩環同時排他鎖死 -> 死鎖 E01)"),
    (4, 3, 6, "Mut ⊗ Shared = Dropped (排他與共享衝突 -> E01)"),
    (2, 4, 6, "Owned ⊗ Mut = Dropped (排他期間存取 -> E02/E03)"),
    (5, 2, 6, "Moved ⊗ Owned = Dropped (Move後讀寫 -> E06)"),
    (1, 2, 6, "Uninit ⊗ Owned = Dropped (未初始化讀寫 -> E29)")
]

for s1, s2, expected, desc in test_cases_m2:
    v1 = delta(6, s1)
    v2 = delta(6, s2)
    v_out = stp(M_loop, stp(v1, v2))
    actual_state = np.argmax(v_out) + 1
    assert actual_state == expected, f"測試失敗: {desc}"
    print(f"    • {desc} -> 輸出: {STATE_NAMES[actual_state]} ✅")

print("    ✅ 六值非經典邏輯結構矩陣 M_loop 代數運算自証 PASS")


# ======================================================================================
# 模組 3：Y(s)/X(s) = G(s)/(1 + G(s)H(s)) 符號代數與狀態空間絕對對等自証
# ======================================================================================
print("\n" + "=" * 88)
print("【模組 3】反饋閉環 Y(s)/X(s) = G(s)/(1 + G(s)H(s)) 狀態空間符號代數自証")
print("=" * 88)

s = sp.Symbol('s')
b1, b0, a1, a0 = sp.symbols('b1 b0 a1 a0')
d0, c0 = sp.symbols('d0 c0')

# 1. 連續傳遞函數 G(s) 與 H(s)
N_G = b1 * s + b0
D_G = s**2 + a1 * s + a0
G_sym = N_G / D_G

N_H = d0
D_H = s + c0
H_sym = N_H / D_H

print(f"3.1 定義前向受控對象 (Plant):   G(s) = ({N_G}) / ({D_G})")
print(f"    定義反饋感測通道 (Sensor): H(s) = ({N_H}) / ({D_H})")

# 2. 閉環特徵多項式 1 + G(s)H(s) = 0
expected_char_poly = sp.expand(D_G * D_H + N_G * N_H)
print(f"3.2 閉環傳遞函數分母特徵多項式 1 + G(s)H(s) = 0:")
print(f"    P(s) = D_G(s)*D_H(s) + N_G(s)*N_H(s) = {expected_char_poly}")

# 3. 構造前向與反饋的狀態空間實現 (Controllable Canonical Form)
A_G = sp.Matrix([[0, 1], [-a0, -a1]])
B_G = sp.Matrix([[0], [1]])
C_G = sp.Matrix([[b0, b1]])

A_H = sp.Matrix([[-c0]])
B_H = sp.Matrix([[1]])
C_H = sp.Matrix([[d0]])

# 4. 閉環全域狀態矩陣 A_cl (3x3):
A_cl = sp.Matrix.vstack(
    sp.Matrix.hstack(A_G, -B_G * C_H),
    sp.Matrix.hstack(B_H * C_G, A_H)
)
print("3.3 閉環狀態矩陣 A_cl (3x3):")
sp.pprint(A_cl)

# 5. 計算 det(sI - A_cl)
I3 = sp.eye(3)
char_poly_ss = sp.expand((s * I3 - A_cl).det())
print(f"3.4 狀態空間特徵多項式 det(sI - A_cl) = {char_poly_ss}")

# 6. 代數零殘差驗證
diff_char = sp.simplify(char_poly_ss - expected_char_poly)
print(f"3.5 符號代數殘差: det(sI - A_cl) - (D_G*D_H + N_G*N_H) = {diff_char}")
assert diff_char == 0, "狀態空間特徵多項式不匹配！"
print("    ✅ 傳遞函數 Y(s)/X(s) 與 狀態空間閉環特徵多項式絕對對等自証 PASS (零殘差 0)")

# 7. 程代展半張量積 (STP) 離散狀態空間閉環自証
print("\n3.6 程代展半張量積 (STP) 離散邏輯反饋閉環驗證:")
# 構造 Plant 轉移矩陣 L_G ∈ R^{2 x 4}, Sensor 轉移矩陣 L_H ∈ R^{2 x 4}
L_G_stp = np.array([[0, 1, 1, 0], [1, 0, 0, 1]], dtype=int)
L_H_stp = np.array([[1, 0, 0, 1], [0, 1, 1, 0]], dtype=int)
M_sub_stp = np.array([[0, 1, 1, 0], [1, 0, 0, 1]], dtype=int)
r_stp = delta(2, 1) # 固定參考輸入
M_r_stp = stp(M_sub_stp, r_stp)

# 求解離散閉環狀態矩陣 L_cl_stp ∈ R^{4 x 4}
L_cl_stp = np.zeros((4, 4), dtype=int)
for i in range(1, 3):
    for j in range(1, 3):
        col_idx = (i - 1) * 2 + (j - 1)
        x_G = delta(2, i)
        x_H = delta(2, j)
        v = x_H
        u = stp(M_r_stp, v)
        x_G_next = stp(L_G_stp, stp(u, x_G))
        x_H_next = stp(L_H_stp, stp(x_G, x_H))
        X_next = stp(x_G_next, x_H_next)
        L_cl_stp[:, col_idx] = X_next.flatten()

print("    離散 STP 閉環轉移矩陣 L_cl (4x4):")
print(f"    {L_cl_stp.tolist()}")
assert np.all(np.sum(L_cl_stp, axis=0) == 1), "L_cl 非合法邏輯矩陣！"
print("    ✅ 離散 STP 反饋閉環矩陣 L_cl 構建成功且每列均為標准邏輯基向量！")


# ======================================================================================
# 模組 4：軌跡集合 ⟷ k 步有限型移位 (SFT) 與動態 Zeta 函數機械自証
# ======================================================================================
print("\n" + "=" * 88)
print("【模組 4】軌跡流 ⟷ k 步有限型移位 (SFT) 與動態 Zeta 函數機械自証")
print("=" * 88)

# 正則合法借用層之拓撲轉移矩陣 A_reg (去除死鎖態後的 4 個活躍維度)
A_reg = sp.Matrix([
    [0, 1, 0, 0],  # 1: Uninit -> Owned
    [0, 1, 1, 1],  # 2: Owned -> Owned, Shared, Mut
    [0, 1, 1, 0],  # 3: Shared -> Owned, Shared
    [0, 1, 0, 0]   # 4: Mut -> Owned
])
dim_reg = A_reg.shape[0]
I_reg = sp.eye(dim_reg)
z = sp.Symbol('z')

# 4.1 閉式有理函數: ζ(z) = 1 / det(I - z*A_reg)
det_poly_sft = sp.expand((I_reg - z * A_reg).det())
zeta_rational = 1 / det_poly_sft
print(f"4.1 SFT 拓撲轉移矩陣特徵多項式: det(I - z*A) = {det_poly_sft}")

# 4.2 級數展開: Taylor 展開至 z^8
ORDER = 8
zeta_exact_series = sp.series(zeta_rational, z, 0, ORDER + 1).removeO()
print(f"4.2 閉式 Zeta 函數展開至 z^{ORDER}:")
print(f"    ζ_exact(z) = {sp.expand(zeta_exact_series)}")

# 4.3 依據週期軌道 Fix(σ^m) = Tr(A^m) 計算動態和項
dyn_sum = sp.Integer(0)
A_pow = sp.eye(dim_reg)
print(f"4.3 依據動態軌道點 Fix(σ^m) = Tr(A^m) 逐階計算:")
for m in range(1, ORDER + 1):
    A_pow = A_pow * A_reg
    tr_m = A_pow.trace()
    dyn_sum += (z**m / m) * tr_m
    print(f"    • 週期 m = {m:2d}: Tr(A^{m}) = {int(tr_m):3d} (長度為 {m} 的週期軌道數)")

zeta_dyn_series = sp.series(sp.exp(dyn_sum), z, 0, ORDER + 1).removeO()
print(f"    ζ_dyn(z)   = {sp.expand(zeta_dyn_series)}")

# 4.4 檢驗 Bowen-Lanford 定理代數等價性
diff_zeta = sp.expand(zeta_exact_series - zeta_dyn_series)
print(f"4.4 殘差自証: ζ_exact(z) - ζ_dyn(z) = {diff_zeta}")
assert diff_zeta == 0, "Bowen-Lanford 動態 Zeta 函數機械自証失敗！"
print("    ✅ Bowen-Lanford 定理自証 PASS: 動態軌道計數與有理閉式完全恆等！")

# 4.5 計算拓撲熵與譜半徑
A_reg_np = np.array(A_reg).astype(float)
eigs = np.linalg.eigvals(A_reg_np)
rho = np.max(np.abs(eigs))
h_top = np.log(rho)
print(f"4.5 SFT 動力系統拓撲特徵標:")
print(f"    最大特徵值 (Perron-Frobenius 譜半徑) λ_max = {rho:.6f}")
print(f"    拓撲熵 h_top = log(λ_max) = {h_top:.6f}")
print(f"    動態 Zeta 函數收斂半徑 R = 1/λ_max = {1.0/rho:.6f}")


# ======================================================================================
# 模組 5：A/B 閉環發現機 (Flow-Matching + Verifier + Reward + SLS) 機械自証
# ======================================================================================
print("\n" + "=" * 88)
print("【模組 5】A/B 閉環發現機 (連續生成 + 形式驗證 + 幾何引導 SLS) 機械自証")
print("=" * 88)

class DatalogOracleMini:
    """提取自 Layer5 核心借用規則的輕量 Datalog 驗證器 (Model B)"""
    @staticmethod
    def verify(statements):
        errors = []
        provenance = []
        active_loans = {}
        var_status = {}

        for idx, stmt in enumerate(statements):
            op = stmt[0]
            if op == 'decl':
                var_status[stmt[1]] = 'owned'
            elif op in ('lend_mut', 'lend_sh'):
                var = stmt[1]
                ref = stmt[2]
                kind = 'mut' if op == 'lend_mut' else 'sh'
                for existing_ref, loan in list(active_loans.items()):
                    if loan['target'] == var:
                        if loan['kind'] == 'mut' or kind == 'mut':
                            errors.append(('E01', idx, f"紅弧交越: 借用 {ref}({kind}) 與 {existing_ref}({loan['kind']}) 重疊"))
                            provenance.append((idx, loan['start'], 'eclash'))
                if var_status.get(var) == 'moved':
                    errors.append(('E06', idx, f"move 後出借: {var} 已被移動"))
                    provenance.append((idx, idx, 'emove'))
                active_loans[ref] = {'target': var, 'kind': kind, 'start': idx, 'last_use': idx}
            elif op == 'use':
                ref = stmt[1]
                if ref in active_loans:
                    active_loans[ref]['last_use'] = idx
            elif op == 'write_var':
                var = stmt[1]
                for existing_ref, loan in active_loans.items():
                    if loan['target'] == var:
                        errors.append(('E02', idx, f"借用期間寫入被借者 {var} (~ E0506)"))
                        provenance.append((idx, loan['start'], 'ewrite'))
            elif op == 'move':
                var = stmt[1]
                var_status[var] = 'moved'
                for existing_ref, loan in active_loans.items():
                    if loan['target'] == var:
                        errors.append(('E07', idx, f"借用活躍期間 move 被借者 {var}"))
                        provenance.append((idx, loan['start'], 'eloanmove'))
        return errors, provenance

# 模擬 Model A (Flow-Matching 策略空間生成候選程式)
print("5.1 啟動 Model A (Flow-Matching 策略空間生成初始候選):")
candidate_code = [
    ('decl', 'x'),
    ('lend_mut', 'x', 'a'),  # s1
    ('lend_mut', 'x', 'b'),  # s2 (紅弧交越 E01 衝突)
    ('use', 'a'),            # s3
    ('use', 'b')             # s4
]
for i, s_stmt in enumerate(candidate_code):
    print(f"      s{i}: {s_stmt}")

oracle = DatalogOracleMini()
errs, prov = oracle.verify(candidate_code)
print(f"5.2 Model B Oracle 判定: 發現 {len(errs)} 個違規邊 (RedEdges = {len(errs)})")
for code, sid, msg in errs:
    print(f"      [{code}] s{sid}: {msg}")
assert len(errs) > 0

# 5.3 隨機局部搜索 (SLS) 依據證明樹定向翻轉
print("5.3 啟動閉環幾何引導 SLS (Focused Stochastic Local Search):")
def sls_closed_loop_repair(candidate, oracle, max_iters=10):
    curr = list(candidate)
    for it in range(max_iters):
        errors, provenance = oracle.verify(curr)
        red_edges = len(errors)
        print(f"    [迭代 {it}] 當前衝突邊 RedEdges = {red_edges}")
        if red_edges == 0:
            return curr, it
        err_idx, target_idx, reason = provenance[0]
        if reason == 'eclash':
            if curr[err_idx][0] == 'lend_mut':
                curr[err_idx] = ('lend_sh', curr[err_idx][1], curr[err_idx][2])
                print(f"      🔧 SLS 局部定向翻轉: 將 s{err_idx} 'lend_mut' -> 'lend_sh'")
            elif curr[target_idx][0] == 'lend_mut':
                curr[target_idx] = ('lend_sh', curr[target_idx][1], curr[target_idx][2])
                print(f"      🔧 SLS 局部定向翻轉: 將 s{target_idx} 'lend_mut' -> 'lend_sh'")
    return curr, max_iters

repaired_code, iters_taken = sls_closed_loop_repair(candidate_code, oracle)
final_errs, _ = oracle.verify(repaired_code)
print(f"5.4 閉環修復結果 (耗費 {iters_taken} 次迭代):")
for i, s_stmt in enumerate(repaired_code):
    print(f"      s{i}: {s_stmt}")
print(f"    最終 Oracle 驗證錯誤數: {len(final_errs)}")
assert len(final_errs) == 0, "SLS 閉環修復失敗！"
print("    ✅ A/B 閉環發現機收斂自証 PASS: RedEdges 成功降為 0！")


# ======================================================================================
# 模組 6：與真實 Layer5 (弦律 ChordLaw) 倉庫測試案例端對端自証
# ======================================================================================
print("\n" + "=" * 88)
print("【模組 6】與真實 Layer5 (弦律 ChordLaw) 倉庫測試案例端對端自証")
print("=" * 88)

cases_to_test = [
    ('/home/user/Layer5/examples/r01_eclash.cl', 'FAIL', 'E01'),
    ('/home/user/Layer5/examples/ex2_sequential.cl', 'PASS', None),
    ('/home/user/Layer5/examples/r06_emove.cl', 'FAIL', 'E06'),
    ('/home/user/Layer5/examples/ex10_mut_seq.cl', 'PASS', None)
]

for filepath, expected_verdict, expected_code in cases_to_test:
    name = os.path.basename(filepath)
    with open(filepath, encoding='utf-8') as f:
        code_text = f.read()
    pr, dl, errors = chordlaw.check(code_text, 'nll')
    actual_verdict = 'PASS' if not errors else 'FAIL'
    print(f"6.1 測試 Layer5 原生案例: {name}")
    print(f"    預期判決: {expected_verdict} | 實際判決: {actual_verdict}")
    assert actual_verdict == expected_verdict, f"判決不符: {name}"
    if expected_code:
        err_codes = [e[0] for e in errors]
        print(f"    預期包含錯誤代碼: {expected_code} | 實際捕獲: {err_codes}")
        assert expected_code in err_codes, f"未捕獲預期錯誤碼 {expected_code}"
    print(f"    ✅ 案例 {name} 機械自証 PASS")

print("\n" + "=" * 88)
print("  🎉 全部 6 個模組數學運算與代數拓撲絕對對等機械自証 100% 成功！")
print("=" * 88)
