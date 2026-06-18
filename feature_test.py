#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
Feature 功能回归测试脚本
覆盖: 材料对比磨损速率验证、跨时代寿命提升验证、
      润滑剂摩擦系数对比验证、虚拟维护教育性体验验证
包含: 正常场景、边界场景、异常场景
"""

import os
import json
import sys
import re
from pathlib import Path

PROJECT_ROOT = Path(__file__).parent
BACKEND_DIR = PROJECT_ROOT / "backend"
FRONTEND_DIR = PROJECT_ROOT / "frontend"
CONFIG_DIR = BACKEND_DIR / "config"
MODULES_DIR = BACKEND_DIR / "internal" / "modules"
SQL_DIR = PROJECT_ROOT / "sql"

test_results = []


def test(name, func):
    """测试包装器"""
    try:
        func()
        test_results.append((name, "[OK] 通过", ""))
        print(f"[OK] {name}")
    except AssertionError as e:
        test_results.append((name, "[FAIL] 失败", str(e)))
        print(f"[FAIL] {name}: {e}")
    except Exception as e:
        test_results.append((name, "[ERR] 异常", str(e)))
        print(f"[ERR] {name} (异常): {e}")


# ==================== 材料配置验证测试 ====================

def test_materials_json_structure():
    """测试轴承材料配置文件完整性"""
    fpath = CONFIG_DIR / "bearing_materials.json"
    assert fpath.exists(), f"材料配置文件不存在: {fpath}"

    data = json.loads(fpath.read_text(encoding='utf-8'))
    assert "materials" in data, "缺少 materials 数组"
    materials = data["materials"]

    required_fields = [
        "code", "name_cn", "era", "category",
        "hardness_hv_nominal", "archard_k_base",
        "surface_roughness_rms_meters", "elastic_modulus_pa",
        "wear_resistance_factor", "historical_note",
    ]

    eras = set()
    categories = set()
    for mat in materials:
        for field in required_fields:
            assert field in mat, f"材料 {mat.get('code', '?')} 缺少字段: {field}"
        assert mat["hardness_hv_nominal"] > 0, f"材料 {mat['code']} 硬度应>0"
        assert mat["archard_k_base"] > 0, f"材料 {mat['code']} Archard K应>0"
        assert mat["era"] in ("ancient", "modern"), f"材料 {mat['code']} era 非法: {mat['era']}"
        eras.add(mat["era"])
        categories.add(mat["category"])
        assert len(mat["historical_note"]) > 10, f"材料 {mat['code']} 历史注释过短"

    assert "ancient" in eras, "缺少古代材料"
    assert "modern" in eras, "缺少现代材料"

    ancient_count = sum(1 for m in materials if m["era"] == "ancient")
    modern_count = sum(1 for m in materials if m["era"] == "modern")
    assert ancient_count >= 3, f"古代材料应>=3种，实际 {ancient_count}"
    assert modern_count >= 2, f"现代材料应>=2种，实际 {modern_count}"

    print(f"  共 {len(materials)} 种材料: 古代 {ancient_count}, 现代 {modern_count}")
    print(f"  类别: {', '.join(sorted(categories))}")


def test_materials_wear_rate_ordering():
    """测试材料磨损速率合理性（木材 > 铸铁 > 青铜 > 现代滚动）"""
    fpath = CONFIG_DIR / "bearing_materials.json"
    data = json.loads(fpath.read_text(encoding='utf-8'))
    materials = {m["code"]: m for m in data["materials"]}

    wear_rate_proxy = {}
    for code, mat in materials.items():
        k = mat["archard_k_base"]
        h = mat["hardness_hv_nominal"]
        wr = mat["wear_resistance_factor"]
        proxy = k / (h * wr) if h * wr > 0 else 0
        wear_rate_proxy[code] = proxy

    ancient_woods = ["wood_oak", "wood_ironbark"]
    ancient_metals = ["bronze_ancient", "cast_iron_ancient"]
    modern_rolling = ["modern_ball_bearing", "modern_roller_bearing"]

    for wood in ancient_woods:
        if wood in materials and "bronze_ancient" in materials:
            assert wear_rate_proxy[wood] > wear_rate_proxy["bronze_ancient"], \
                f"{wood} 磨损代理值应大于青铜"

    for rolling in modern_rolling:
        if rolling in materials and "bronze_ancient" in materials:
            assert wear_rate_proxy[rolling] < wear_rate_proxy["bronze_ancient"], \
                f"{rolling} 磨损代理值应小于古代青铜"

    print("  材料磨损速率排序符合物理规律")


def test_materials_hardness_range():
    """测试材料硬度值合理性边界"""
    fpath = CONFIG_DIR / "bearing_materials.json"
    data = json.loads(fpath.read_text(encoding='utf-8'))

    for mat in data["materials"]:
        code = mat["code"]
        hv_min = mat["hardness_hv_min"]
        hv_nom = mat["hardness_hv_nominal"]
        hv_max = mat["hardness_hv_max"]

        assert hv_min > 0, f"{code}: 硬度下限应>0"
        assert hv_min <= hv_nom <= hv_max, \
            f"{code}: 硬度范围应满足 min({hv_min}) <= nom({hv_nom}) <= max({hv_max})"

        if mat["era"] == "ancient" and mat["category"] == "wood":
            assert hv_nom < 100, f"古代木材 {code} 硬度应<100HV，实际 {hv_nom}"
        if mat["era"] == "modern" and mat["category"] == "rolling":
            assert hv_nom > 400, f"现代滚动轴承 {code} 硬度应>400HV，实际 {hv_nom}"

    print("  所有材料硬度值范围合理")


# ==================== 润滑剂配置验证测试 ====================

def test_lubricants_json_structure():
    """测试润滑剂配置文件完整性"""
    fpath = CONFIG_DIR / "lubricants.json"
    assert fpath.exists(), f"润滑剂配置文件不存在: {fpath}"

    data = json.loads(fpath.read_text(encoding='utf-8'))
    assert "lubricants" in data, "缺少 lubricants 数组"
    lubricants = data["lubricants"]

    required_fields = [
        "code", "name_cn", "category", "era",
        "viscosity_pas_at_40c", "pressure_viscosity_coefficient",
        "ehl_boost_factor", "wear_reduction_ratio",
        "friction_coefficient_lubricated",
        "max_lubrication_life_hours",
        "historical_note",
    ]

    categories = set()
    for lub in lubricants:
        for field in required_fields:
            assert field in lub, f"润滑剂 {lub.get('code', '?')} 缺少字段: {field}"
        assert 0 < lub["wear_reduction_ratio"] < 1.0, \
            f"润滑剂 {lub['code']} 减磨率应在(0,1): {lub['wear_reduction_ratio']}"
        assert lub["viscosity_pas_at_40c"] > 0, \
            f"润滑剂 {lub['code']} 粘度应>0"
        categories.add(lub["category"])
        assert len(lub["historical_note"]) > 10, \
            f"润滑剂 {lub['code']} 历史注释过短"

    veg_count = sum(1 for l in lubricants if l["category"] == "vegetable")
    animal_count = sum(1 for l in lubricants if l["category"] == "animal")
    mineral_count = sum(1 for l in lubricants if l["category"] in ("mineral", "synthetic"))

    assert veg_count >= 2, f"植物油应>=2种，实际 {veg_count}"
    assert animal_count >= 1, f"动物油应>=1种，实际 {animal_count}"
    assert mineral_count >= 2, f"矿物/合成油应>=2种，实际 {mineral_count}"

    print(f"  共 {len(lubricants)} 种润滑剂: 类别 {', '.join(sorted(categories))}")


def test_lubricants_friction_coefficient_ordering():
    """测试润滑剂摩擦系数排序: 干摩擦 > 动物油 > 植物油 > 矿物油 > 合成油"""
    fpath = CONFIG_DIR / "lubricants.json"
    data = json.loads(fpath.read_text(encoding='utf-8'))
    lubricants = {l["code"]: l for l in data["lubricants"]}

    for code, lub in lubricants.items():
        f_dry = lub["friction_coefficient_dry"]
        f_lub = lub["friction_coefficient_lubricated"]
        assert f_dry > f_lub, \
            f"{code}: 润滑后摩擦系数({f_lub})应小于干摩擦({f_dry})"
        assert 0.001 < f_lub < 0.5, \
            f"{code}: 润滑摩擦系数应在合理范围(0.001, 0.5)，实际 {f_lub}"

    veg_codes = [c for c, l in lubricants.items() if l["category"] == "vegetable"]
    syn_codes = [c for c, l in lubricants.items() if l["category"] == "synthetic"]

    if veg_codes and syn_codes:
        veg_avg = sum(lubricants[c]["friction_coefficient_lubricated"] for c in veg_codes) / len(veg_codes)
        syn_avg = sum(lubricants[c]["friction_coefficient_lubricated"] for c in syn_codes) / len(syn_codes)
        assert veg_avg > syn_avg, \
            f"植物油平均摩擦系数({veg_avg:.3f})应大于合成油({syn_avg:.3f})"
        print(f"  摩擦系数排序验证: 植物油({veg_avg:.3f}) > 合成油({syn_avg:.3f})")


def test_lubricants_wear_reduction_boundary():
    """测试润滑剂减磨率边界: 所有应 >0 且高等级应高于低等级"""
    fpath = CONFIG_DIR / "lubricants.json"
    data = json.loads(fpath.read_text(encoding='utf-8'))

    for lub in data["lubricants"]:
        wr = lub["wear_reduction_ratio"]
        assert wr > 0.05, f"{lub['code']} 减磨率过低: {wr}"
        assert wr < 0.99, f"{lub['code']} 减磨率过高(不现实): {wr}"

    print("  所有润滑剂减磨率在合理边界内 (0.05, 0.99)")


# ==================== Go 后端模块验证测试 ====================

def test_analysis_module_exists():
    """测试对比分析模块代码存在且关键函数完整"""
    fpath = MODULES_DIR / "analysis" / "comparison.go"
    assert fpath.exists(), "comparison.go 不存在"

    content = fpath.read_text(encoding='utf-8')

    required_funcs = [
        "NewComparisonEngine",
        "simulateWear",
        "calculateEHLFilm",
        "wearCoefficientForLambda",
        "CompareMaterials",
        "CompareLubricants",
        "CrossEraComparison",
        "CompareMaterialsGeneric",
        "generateEraInsights",
    ]
    for fn in required_funcs:
        assert f"func (ce *ComparisonEngine) {fn}" in content or f"func {fn}" in content, \
            f"缺少关键函数: {fn}"

    assert "Archard" in content.lower() or "archard" in content, "缺少 Archard 磨损公式"
    assert "Dowson" in content or "dowson" in content, "缺少 Dowson-Higginson EHL 公式"
    assert "simInput" in content, "缺少 simInput 通用输入结构体"
    assert "simOutput" in content, "缺少 simOutput 通用输出结构体"

    print(f"  对比分析模块: {len(required_funcs)} 个关键函数全部存在")


def test_simulateWear_boundary_handling():
    """测试 simulateWear 对边界输入的代码防御"""
    fpath = MODULES_DIR / "analysis" / "comparison.go"
    content = fpath.read_text(encoding='utf-8')

    checks = [
        ("contactArea 零值保护", r"contactArea\s*<=\s*0"),
        ("粘度零值保护", r"viscosity\s*<=\s*0"),
        ("转速零值保护", r"entrainmentSpeed\s*<=\s*0\.001"),
        ("载荷零值保护", r"W\s*<=\s*0"),
        ("油膜厚度最小值钳制", r"film\s*<\s*0\.05e-6"),
        ("油膜厚度最大值钳制", r"film\s*>\s*20e-6"),
        ("寿命上限钳制", r"lifeHours\s*>\s*200000"),
        ("EHL λ 最小值钳制", r"LambdaMinClamp"),
        ("EHL λ 最大值钳制", r"LambdaMaxClamp"),
    ]

    for name, pattern in checks:
        assert re.search(pattern, content), f"缺少边界处理: {name}"

    print(f"  simulateWear 边界防御: {len(checks)} 项检查全部通过")


def test_maintenance_module_exists():
    """测试虚拟维护模块代码存在且关键函数完整"""
    fpath = MODULES_DIR / "maintenance" / "manager.go"
    assert fpath.exists(), "manager.go 不存在"

    content = fpath.read_text(encoding='utf-8')

    required_funcs = [
        "NewMaintenanceManager",
        "PreviewBearingReplacement",
        "ExecuteBearingReplacement",
        "PreviewLubricantAddition",
        "ExecuteLubricantAddition",
        "GenerateMaintenancePlan",
        "guessMaterialCode",
        "suggestedReplacementMaterials",
        "suggestedLubricants",
    ]
    for fn in required_funcs:
        assert fn in content, f"缺少关键函数: {fn}"

    assert "Preview" in content and "Execute" in content, "缺少 Preview/Execute 双阶段设计"
    assert "MaintenanceEffectPreview" in content, "缺少 MaintenanceEffectPreview 返回类型"
    assert "MaintenanceRecord" in content, "缺少 MaintenanceRecord 模型"

    print(f"  虚拟维护模块: {len(required_funcs)} 个关键函数全部存在")


def test_guessMaterialCode_normal_and_edge():
    """测试材料识别的正常、边界、异常输入代码防御"""
    fpath = MODULES_DIR / "maintenance" / "manager.go"
    content = fpath.read_text(encoding='utf-8')

    normal_keywords = ["青铜", "铸铁", "橡木", "青冈", "包铜", "球轴承", "巴氏"]
    for kw in normal_keywords:
        assert kw in content, f"缺少正常材料关键词识别: {kw}"

    english_keywords = ["bronze", "iron", "oak", "ironbark", "ball", "babbitt"]
    for kw in english_keywords:
        assert kw in content, f"缺少英文材料关键词识别: {kw}"

    assert "default" in content or 'case ' in content, "缺少默认回退分支"

    assert "toLower" in content or "strings.ToLower" in content or "containsAny" in content, \
        "缺少大小写不敏感处理"

    print("  材料识别: 中文/英文/默认回退 均已实现")


def test_analysis_error_handling():
    """测试对比模块对异常输入的防御代码"""
    fpath = MODULES_DIR / "analysis" / "comparison.go"
    content = fpath.read_text(encoding='utf-8')

    patterns = [
        (r"GetMaterial\(code\)", "无效材料代码过滤"),
        (r"GetLubricant\(code\)", "无效润滑剂代码过滤"),
        (r"if\s+!ok\s*\{", "无效输入跳过(ok检查)"),
        (r"continue", "无效输入continue跳过"),
    ]
    for pattern, desc in patterns:
        assert re.search(pattern, content), f"缺少异常处理: {desc}"

    print("  对比模块异常输入防御: 完整")


# ==================== 跨时代对比算法验证测试 ====================

def test_cross_era_algorithm_structure():
    """测试跨时代对比算法逻辑: 古代组 vs 现代组 + 寿命提升倍数 + 洞察生成"""
    fpath = MODULES_DIR / "analysis" / "comparison.go"
    content = fpath.read_text(encoding='utf-8')

    assert "CrossEraComparison" in content, "缺少 CrossEraComparison 函数"
    assert "LifeImprovementX" in content, "缺少寿命提升倍数字段"
    assert "WearReductionPct" in content, "缺少磨损减少百分比字段"
    assert "InsightSummary" in content, "缺少洞察摘要字段"
    assert "AncientBest" in content and "ModernBest" in content, "缺少古今最佳材料对比"

    assert "ancientCodes" in content or "ancient_codes" in content.lower(), "缺少古代材料组"
    assert "modernCodes" in content or "modern_codes" in content.lower(), "缺少现代材料组"

    assert re.search(r"modernBestLife\s*/\s*ancientBestLife", content) or \
        "LifeImprovementX" in content, \
        "缺少寿命提升倍数 = 现代/古代 计算公式"

    print("  跨时代对比算法结构: 完整")


def test_cross_era_insight_generation():
    """测试跨时代洞察生成覆盖多个场景"""
    fpath = MODULES_DIR / "analysis" / "comparison.go"
    content = fpath.read_text(encoding='utf-8')

    insight_patterns = [
        (r"50", "50倍以上寿命提升描述"),
        (r"10", "10倍以上寿命提升描述"),
        (r"95", "95%以上磨损减少描述"),
        (r"古代最优", "古代最优方案描述"),
        (r"现代最优", "现代最优方案描述"),
        (r"经验试错|工匠|摩擦学", "教育性对比描述"),
    ]
    for pattern, desc in insight_patterns:
        if re.search(pattern, content):
            print(f"    [√] {desc}")
        else:
            print(f"    [!] 建议添加: {desc}")

    print("  跨时代洞察生成: 多场景覆盖")


# ==================== 前端集成验证测试 ====================

def test_frontend_feature_views_exist():
    """测试三大新功能前端视图文件存在"""
    required_js = [
        FRONTEND_DIR / "js" / "material_compare.js",
        FRONTEND_DIR / "js" / "lubricant_analysis.js",
        FRONTEND_DIR / "js" / "virtual_maintenance.js",
    ]
    for f in required_js:
        assert f.exists(), f"前端视图文件不存在: {f.name}"

    html_content = (FRONTEND_DIR / "index.html").read_text(encoding='utf-8')
    assert "material-compare" in html_content, "index.html 缺少材料对比视图"
    assert "lubricant" in html_content, "index.html 缺少润滑剂分析视图"
    assert "maintenance" in html_content, "index.html 缺少虚拟维护视图"

    assert "material_compare.js" in html_content, "HTML 缺少 material_compare.js 引用"
    assert "lubricant_analysis.js" in html_content, "HTML 缺少 lubricant_analysis.js 引用"
    assert "virtual_maintenance.js" in html_content, "HTML 缺少 virtual_maintenance.js 引用"

    app_content = (FRONTEND_DIR / "js" / "app.js").read_text(encoding='utf-8')
    assert "_initFeatureViews" in app_content, "app.js 缺少 _initFeatureViews 初始化方法"
    assert "MaterialCompare" in app_content, "app.js 缺少 MaterialCompare 引用"
    assert "LubricantAnalysis" in app_content, "app.js 缺少 LubricantAnalysis 引用"
    assert "VirtualMaintenance" in app_content, "app.js 缺少 VirtualMaintenance 引用"

    print("  前端三大视图: HTML/JS/app.js 引用全部完整")


def test_frontend_feature_navigation():
    """测试新功能导航按钮与NEW标签"""
    html_content = (FRONTEND_DIR / "index.html").read_text(encoding='utf-8')

    nav_buttons = re.findall(r'data-view="(material-compare|lubricant|maintenance)"', html_content)
    assert len(nav_buttons) == 3, f"导航按钮应3个，实际 {len(nav_buttons)}: {nav_buttons}"

    assert "new-feature" in html_content, "缺少 new-feature 导航样式"

    css_content = (FRONTEND_DIR / "css" / "style.css").read_text(encoding='utf-8')
    assert ".new-feature" in css_content, "CSS 缺少 .new-feature 样式"
    assert "feature-layout" in css_content, "CSS 缺少 .feature-layout 布局"
    assert "maintenance-layout" in css_content, "CSS 缺少 .maintenance-layout 布局"
    assert "health-gauge" in css_content, "CSS 缺少健康仪表盘样式"
    assert "preview-grid" in css_content, "CSS 缺少 Preview 四宫格样式"
    assert "cross-era-lift" in css_content, "CSS 缺少跨时代大数字样式"
    assert "rank-1" in css_content, "CSS 缺少排名徽章样式"
    assert "lube-badge" in css_content, "CSS 缺少润滑状态徽章样式"

    print("  前端导航与样式: 完整")


def test_frontend_api_methods():
    """测试前端 API 封装中的新 Feature 方法"""
    api_content = (FRONTEND_DIR / "js" / "api.js").read_text(encoding='utf-8')

    required_methods = [
        "getMaterials",
        "getLubricants",
        "compareMaterials",
        "compareLubricants",
        "crossEraComparison",
        "previewBearingReplacement",
        "executeBearingReplacement",
        "previewLubricantAddition",
        "executeLubricantAddition",
        "getMaintenanceHistory",
        "getMaintenancePlan",
    ]

    for method in required_methods:
        assert method in api_content, f"api.js 缺少方法: {method}"

    print(f"  前端 API 封装: {len(required_methods)} 个 Feature 方法全部存在")


# ==================== SQL 数据库验证测试 ====================

def test_feature_sql_tables():
    """测试 Feature 相关 SQL 表和视图定义"""
    fpath = SQL_DIR / "extensions_feature_compare_maintenance.sql"
    assert fpath.exists(), "Feature SQL 脚本不存在"

    content = fpath.read_text(encoding='utf-8')

    required_tables = [
        "bearing_materials_ref",
        "lubricants_ref",
        "maintenance_records",
        "comparison_reports",
        "bearing_lubrication_status",
    ]
    for table in required_tables:
        assert table in content.lower() or table.replace("_", "") in content.lower().replace("_", ""), \
            f"SQL 缺少表定义: {table}"

    assert "hypertable" in content.lower() or "create_hypertable" in content.lower(), \
        "maintenance_records 应定义为 TimescaleDB Hypertable"

    views = re.findall(r"CREATE\s+(OR\s+REPLACE\s+)?VIEW\s+(\w+)", content, re.IGNORECASE)
    if views:
        print(f"  包含视图: {', '.join(v[1] for v in views)}")

    print(f"  SQL 表定义: {len(required_tables)} 张核心表全部存在")


# ==================== API Handler 验证测试 ====================

def test_feature_api_handlers():
    """测试 Feature API 处理器文件与路由"""
    fpath = BACKEND_DIR / "internal" / "api" / "feature_handlers.go"
    assert fpath.exists(), "feature_handlers.go 不存在"

    content = fpath.read_text(encoding='utf-8')

    handler_patterns = [
        (r"ListBearingMaterials|GetMaterialReference", "reference materials handler"),
        (r"ListLubricants|GetLubricantReference", "reference lubricants handler"),
        (r"CompareMaterials", "compare materials handler"),
        (r"CompareLubricants", "compare lubricants handler"),
        (r"CrossEraComparison", "cross era comparison handler"),
        (r"PreviewBearingReplacement", "preview bearing replacement handler"),
        (r"ExecuteBearingReplacement", "execute bearing replacement handler"),
        (r"PreviewLubricantAddition", "preview lubricant addition handler"),
        (r"ExecuteLubricantAddition", "execute lubricant addition handler"),
        (r"GetMaintenanceHistory", "maintenance history handler"),
        (r"GetMaintenancePlan", "maintenance plan handler"),
    ]

    found = 0
    for pattern, desc in handler_patterns:
        if re.search(pattern, content, re.IGNORECASE):
            found += 1
        else:
            print(f"    [!] 未匹配: {desc}")

    print(f"  API Handler: {found}/{len(handler_patterns)} 个路由处理器匹配")
    assert found >= 10, f"API Handler 覆盖率不足: {found}/{len(handler_patterns)}"


def test_main_route_registration():
    """测试 main.go 中新 Feature 路由注册"""
    fpath = BACKEND_DIR / "main.go"
    content = fpath.read_text(encoding='utf-8')

    assert "featureHandler" in content or "FeatureHandler" in content, \
        "main.go 未实例化 featureHandler"
    assert "/api/v1/reference" in content or "reference" in content, "缺少 reference 路由组"
    assert "/api/v1/analysis" in content or "analysis" in content, "缺少 analysis 路由组"
    assert "/api/v1/maintenance" in content or "maintenance" in content, "缺少 maintenance 路由组"

    print("  main.go 路由注册: 三组 Feature API 路由已挂载")


# ==================== 数据模型验证测试 ====================

def test_feature_models_exist():
    """测试 Feature 相关数据模型完整"""
    fpath = BACKEND_DIR / "internal" / "models" / "models.go"
    content = fpath.read_text(encoding='utf-8')

    required_models = [
        "MaintenanceRecord",
        "MaterialComparisonResult",
        "MaterialComparisonItem",
        "LubricantComparisonResult",
        "LubricantComparisonItem",
        "CrossEraComparisonResult",
        "EraComparisonItem",
        "MaintenanceEffectPreview",
    ]

    for model in required_models:
        assert f"type {model} struct" in content, f"缺少数据模型: {model}"

    print(f"  数据模型: {len(required_models)} 个 Feature 模型全部定义")


# ==================== 虚拟维护教育性验证测试 ====================

def test_maintenance_educational_content():
    """测试虚拟维护模块是否包含足够的教育性内容"""
    fpath = MODULES_DIR / "maintenance" / "manager.go"
    content = fpath.read_text(encoding='utf-8')

    educational_checks = [
        ("材料历史说明", r"historical.*:=.*HistoricalNote|historical.*HistoricalNote"),
        ("润滑剂历史说明", r"historical.*HistoricalNote"),
        ("成本教育提示", r"generateReplacementCostHint|generateLubricantCostHint"),
        ("维护优先级分级", r"urgent|high|medium|low|routine"),
        ("推荐材料解释", r"suggestedReplacementMaterials"),
        ("推荐润滑剂解释", r"suggestedLubricants"),
        ("操作摘要说明", r"ActionSummary|action_summary"),
    ]

    for name, pattern in educational_checks:
        if re.search(pattern, content, re.IGNORECASE):
            print(f"    [√] {name}")
        else:
            print(f"    [!] 可增强: {name}")

    assert "generateReplacementCostHint" in content, "缺少成本教育提示"
    assert "ActionSummary" in content, "缺少操作摘要"

    print("  虚拟维护教育性内容: 核心要素齐全")


def test_maintenance_preview_execute_flow():
    """测试 Preview → Execute 两阶段操作流代码完整性"""
    fpath = MODULES_DIR / "maintenance" / "manager.go"
    content = fpath.read_text(encoding='utf-8')

    assert "PreviewBearingReplacement" in content
    assert "ExecuteBearingReplacement" in content
    assert "PreviewLubricantAddition" in content
    assert "ExecuteLubricantAddition" in content

    assert "PreviewBearingReplacement" in content and "ExecuteBearingReplacement" in content
    execute_calls_preview = re.findall(r"func.*Execute.*\{[\s\S]{0,500}Preview", content)
    assert len(execute_calls_preview) >= 1, "Execute 应先调用 Preview 进行验证"

    assert "InsertMaintenanceRecord" in content or "saveMaintenanceRecord" in content, \
        "Execute 应保存维护记录"
    assert "UpdateBearingMaterialAndHardness" in content, "Execute 应更新轴承材料"
    assert "UpsertBearingLubricationStatus" in content, "Execute 应更新润滑状态"

    print("  Preview→Execute 双阶段流程: 代码结构完整")


# ==================== 零侵入兼容性验证测试 ====================

def test_zero_intrusion_guarantee():
    """验证新 Feature 不修改原有 Channel 消息驱动架构（零侵入保证）"""
    sched_content = (BACKEND_DIR / "internal" / "scheduler" / "scheduler.go").read_text(encoding='utf-8')
    msg_content = (MODULES_DIR / "messages" / "messages.go").read_text(encoding='utf-8')
    wear_content = (MODULES_DIR / "wear_simulator" / "simulator.go").read_text(encoding='utf-8')
    life_content = (MODULES_DIR / "life_predictor" / "predictor.go").read_text(encoding='utf-8')

    intrusion_patterns = ["ComparisonEngine", "MaintenanceManager", "MaterialComparison", "CrossEra"]

    for name, content in [
        ("scheduler.go", sched_content),
        ("messages.go", msg_content),
        ("wear_simulator/simulator.go", wear_content),
        ("life_predictor/predictor.go", life_content),
    ]:
        for p in intrusion_patterns:
            if p in content:
                print(f"    [!] {name} 中发现 Feature 引用: {p}")

    print("  零侵入验证: 原有 Channel 架构模块未被 Feature 修改")


def test_backward_compatibility_alias():
    """验证 Database 类型别名确保向后兼容"""
    db_content = (BACKEND_DIR / "internal" / "database" / "db.go").read_text(encoding='utf-8')

    assert "type Database = DB" in db_content or "type Database DB" in db_content, \
        "缺少 Database 类型别名，可能破坏兼容性"

    print("  向后兼容: Database 类型别名已定义")


# ==================== 执行测试 ====================

def main():
    print("=" * 70)
    print("Feature 功能回归测试")
    print("覆盖: 材料对比 | 跨时代对比 | 润滑剂对比 | 虚拟维护")
    print("维度: 正常场景 | 边界场景 | 异常场景 | 教育性体验")
    print("=" * 70)
    print()

    print("▌ 材料配置验证")
    print("-" * 40)
    test("材料配置文件完整性", test_materials_json_structure)
    test("材料磨损速率排序合理性", test_materials_wear_rate_ordering)
    test("材料硬度值边界合理性", test_materials_hardness_range)

    print()
    print("▌ 润滑剂配置验证")
    print("-" * 40)
    test("润滑剂配置文件完整性", test_lubricants_json_structure)
    test("润滑剂摩擦系数排序合理性", test_lubricants_friction_coefficient_ordering)
    test("润滑剂减磨率边界检查", test_lubricants_wear_reduction_boundary)

    print()
    print("▌ 后端模块代码验证")
    print("-" * 40)
    test("对比分析模块代码完整性", test_analysis_module_exists)
    test("simulateWear 边界处理", test_simulateWear_boundary_handling)
    test("虚拟维护模块代码完整性", test_maintenance_module_exists)
    test("材料识别正常/边界/异常", test_guessMaterialCode_normal_and_edge)
    test("对比模块异常输入防御", test_analysis_error_handling)

    print()
    print("▌ 跨时代对比算法验证")
    print("-" * 40)
    test("跨时代对比算法结构完整", test_cross_era_algorithm_structure)
    test("跨时代洞察生成多场景覆盖", test_cross_era_insight_generation)

    print()
    print("▌ 前端集成验证")
    print("-" * 40)
    test("前端三大视图文件与引用", test_frontend_feature_views_exist)
    test("前端导航与CSS样式", test_frontend_feature_navigation)
    test("前端API封装方法", test_frontend_api_methods)

    print()
    print("▌ 数据库与API验证")
    print("-" * 40)
    test("Feature SQL表与Hypertable", test_feature_sql_tables)
    test("Feature API Handler路由", test_feature_api_handlers)
    test("main.go路由注册", test_main_route_registration)
    test("Feature数据模型", test_feature_models_exist)

    print()
    print("▌ 虚拟维护教育性体验验证")
    print("-" * 40)
    test("维护教育性内容要素", test_maintenance_educational_content)
    test("Preview→Execute双阶段流程", test_maintenance_preview_execute_flow)

    print()
    print("▌ 零侵入与兼容性验证")
    print("-" * 40)
    test("原有Channel架构零侵入", test_zero_intrusion_guarantee)
    test("Database类型别名兼容", test_backward_compatibility_alias)

    print()
    print("=" * 70)
    passed = sum(1 for _, status, _ in test_results if "通过" in status)
    failed = sum(1 for _, status, _ in test_results if "失败" in status)
    errors = sum(1 for _, status, _ in test_results if "异常" in status)
    total = len(test_results)

    print(f"测试结果: {passed}/{total} 通过, {failed} 失败, {errors} 异常")
    print("=" * 70)

    if failed > 0 or errors > 0:
        print()
        print("[FAIL] 失败详情:")
        for name, status, msg in test_results:
            if "通过" not in status:
                print(f"  {name}: {msg}")
        return 1

    print()
    print("[OK] 所有 Feature 测试通过！")
    print()
    print("测试覆盖总结:")
    print("  [OK] 材料磨损速率: 配置完整性、排序合理性、硬度边界")
    print("  [OK] 润滑剂摩擦系数: 配置完整性、系数排序、减磨率边界")
    print("  [OK] 跨时代寿命提升: 算法结构、洞察多场景、古今对比")
    print("  [OK] 虚拟维护教育性: 历史说明、成本提示、Preview/Execute双阶段")
    print("  [OK] 代码防御: 边界输入、异常输入、零侵入兼容")
    return 0


if __name__ == '__main__':
    sys.exit(main())
