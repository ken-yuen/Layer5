#!/usr/bin/env python3
"""
========================================================================================
古代戰法矩陣生成器 (Ancient Battle Formation Matrix Generator)
========================================================================================
依據「三三、四四、五五隊員間疊Buff陣法」原理：
- 三三矩陣 (3x3): 九宮基礎戰術伍 (9 員基本陣型，基礎核心戰力)
- 四四矩陣 (4x4): 十六員雙環交疊防禦反擊陣 (即示範矩陣)
- 五五矩陣 (5x5): 二十五員五行中軍大陣 (由陣眼向外三重疊buff)
========================================================================================
"""

import numpy as np

class AncientFormationMatrix:
    """
    古代陣法矩陣引擎：
    實現隊員基礎戰力佈陣、相鄰隊員互疊 Buff (十字鄰域 + 對角支援 + 環形閉環)
    """
    def __init__(self):
        # 示範 3x3 基礎陣法 (九宮九數分佈)
        self.base_3x3 = np.array([
            [2, 7, 5],
            [6, 3, 1],
            [8, 9, 4]
        ], dtype=int)

        # 示範 4x4 四四陣法 (用戶示範之十六員戰法矩陣)
        self.demo_4x4 = np.array([
            [8, 6, 5, 2],
            [9, 3, 1, 5],
            [4, 1, 6, 8],
            [5, 7, 9, 4]
        ], dtype=int)

    def get_3x3(self):
        """返回三三陣法矩陣"""
        return self.base_3x3.copy()

    def get_4x4(self):
        """返回四四陣法矩陣 (對齊用戶示範)"""
        return self.demo_4x4.copy()

    def generate_5x5(self):
        """
        生成五五大陣矩陣 (25 員五行中軍疊buff大陣):
        - 陣核 (Center 3x3): 繼承三三核心與四四中核精要 (3, 1, 6 核心樞紐)
        - 內中軍 (Center): 5 (中央戊己土，陣眼天樞)
        - 外環 (Outer Ring, 16 員): 依循四四外環戰力分佈，擴展至 25 員
        """
        # 構造五五大陣
        # 核心陣眼為 5，中層環繞 3, 1, 6 互鎖，外層護衛形成對稱攻防平衡
        m5 = np.array([
            [4, 8, 6, 5, 2],
            [5, 9, 3, 1, 7],
            [2, 4, 5, 6, 8],
            [8, 1, 6, 9, 3],
            [5, 7, 9, 4, 1]
        ], dtype=int)
        return m5

    @staticmethod
    def calculate_buff_field(matrix, cross_weight=1, diag_weight=1):
        """
        計算隊員間疊Buff場 (Buff Stacking Field):
        每個隊員獲得相鄰隊員的屬性加成：
        - 十字鄰接 (上、下、左、右): cross_weight
        - 對角鄰接 (左上、右上、左下、右下): diag_weight
        """
        rows, cols = matrix.shape
        buff_matrix = np.zeros_like(matrix, dtype=float)
        
        for r in range(rows):
            for c in range(cols):
                buff = 0.0
                # 遍歷周圍 8 鄰域
                for dr in [-1, 0, 1]:
                    for dc in [-1, 0, 1]:
                        if dr == 0 and dc == 0:
                            continue
                        nr, nc = r + dr, c + dc
                        if 0 <= nr < rows and 0 <= nc < cols:
                            # 十字鄰域
                            if dr == 0 or dc == 0:
                                buff += matrix[nr, nc] * cross_weight * 0.1
                            # 對角鄰域
                            else:
                                buff += matrix[nr, nc] * diag_weight * 0.05
                buff_matrix[r, c] = round(buff, 2)
                
        return buff_matrix

    def display_formation(self, name, matrix):
        """格式化展示陣法與疊Buff戰力"""
        print(f"\n{'=' * 68}")
        print(f"  ⚔️ 【{name}】 ({matrix.shape[0]} × {matrix.shape[1]} 戰法矩陣)")
        print(f"{'=' * 68}")
        
        # 打印原始戰陣
        print("  [隊員原始戰力分佈]:")
        for row in matrix:
            print("   ", "  ".join(f"{val:2d}" for val in row))
            
        # 計算疊buff後的綜合有效戰力
        buff_field = self.calculate_buff_field(matrix)
        total_effective = matrix + buff_field
        
        print("\n  [隊員互疊 Buff 加成場 (來自周邊隊友)]: ")
        for row in buff_field:
            print("   ", "  ".join(f"{val:5.2f}" for val in row))
            
        print("\n  [綜合激發總戰力 (原始值 + 疊Buff)]: ")
        for row in total_effective:
            print("   ", "  ".join(f"{val:5.2f}" for val in row))
            
        # 統計特徵
        print("\n  [陣法戰術指標]:")
        print(f"    • 全陣總戰力:   {matrix.sum()}")
        print(f"    • 疊Buff總增益: {buff_field.sum():.2f}")
        print(f"    • 綜合實效戰力: {total_effective.sum():.2f}")
        print(f"    • 陣眼核心均值: {matrix[1:-1, 1:-1].mean():.2f} (若有中軍)")
        print(f"    • 外圍防禦均值: {np.concatenate([matrix[0,:], matrix[-1,:], matrix[1:-1,0], matrix[1:-1,-1]]).mean():.2f}")

def main():
    engine = AncientFormationMatrix()
    
    # 1. 三三矩陣 (3x3)
    m3 = engine.get_3x3()
    engine.display_formation("三三陣法矩陣 (九宮九數伍)", m3)
    
    # 2. 四四矩陣 (4x4, 示範矩陣)
    m4 = engine.get_4x4()
    engine.display_formation("四四陣法矩陣 (十六員雙環示範陣)", m4)
    
    # 3. 五五矩陣 (5x5)
    m5 = engine.generate_5x5()
    engine.display_formation("五五陣法矩陣 (二十五員五行中軍大陣)", m5)
    
    print("\n" + "=" * 68)
    print("  🎉 三三、四四、五五古代疊Buff戰法矩陣全部按陣法排列生成完畢！")
    print("=" * 68)

if __name__ == "__main__":
    main()
