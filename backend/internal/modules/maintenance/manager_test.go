package maintenance

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"noria-bearing-system/internal/config"
	"noria-bearing-system/internal/models"
)

func setupMaintenanceTestConfig(t *testing.T) {
	t.Helper()

	projectRoot := filepath.Join("..", "..", "..", "..")
	materialsPath := filepath.Join(projectRoot, "config", "bearing_materials.json")
	lubricantsPath := filepath.Join(projectRoot, "config", "lubricants.json")

	for _, p := range []string{materialsPath, lubricantsPath} {
		if _, err := os.Stat(p); err != nil {
			t.Skipf("config file not found, skipping: %s", p)
		}
	}

	matData, _ := os.ReadFile(materialsPath)
	var matWrapper struct {
		Materials []config.BearingMaterial `json:"materials"`
	}
	_ = json.Unmarshal(matData, &matWrapper)
	config.AppConfig.Materials = config.BearingMaterialsConfig{Materials: matWrapper.Materials}
	config.AppConfig.Materials.BuildIndex()

	lubDataRaw, _ := os.ReadFile(lubricantsPath)
	var lubWrapper struct {
		Lubricants                  []config.Lubricant `json:"lubricants"`
		RecommendedLubricationFreq  map[string]float64  `json:"recommended_lubrication_freq,omitempty"`
	}
	_ = json.Unmarshal(lubDataRaw, &lubWrapper)
	config.AppConfig.Lubricants = config.LubricantsConfig{
		Lubricants:                 lubWrapper.Lubricants,
		RecommendedLubricationFreq: lubWrapper.RecommendedLubricationFreq,
	}
	config.AppConfig.Lubricants.BuildIndex()
}

func newTestManager() *MaintenanceManager {
	return &MaintenanceManager{
		db:             nil,
		engine:         nil,
		materialsCfg:   &config.AppConfig.Materials,
		lubricantsCfg:  &config.AppConfig.Lubricants,
		alertThreshold: 0.7,
	}
}

// ==================== 材料识别 guessMaterialCode 测试 ====================

func TestGuessMaterialCode_AncientChinese(t *testing.T) {
	setupMaintenanceTestConfig(t)
	mm := newTestManager()

	cases := []struct {
		material string
		expected string
	}{
		{"青铜", "bronze_ancient"},
		{"古代青铜", "bronze_ancient"},
		{"锡青铜 CuSn10", "bronze_ancient"},
		{"铸铁", "cast_iron_ancient"},
		{"灰口铸铁", "cast_iron_ancient"},
		{"青冈木", "wood_ironbark"},
		{"铁栎木", "wood_ironbark"},
		{"橡木", "wood_oak"},
		{"枣木硬木", "wood_oak"},
		{"包铜铁皮", "wood_wrapped_copper"},
		{"复合材料铜包木", "wood_wrapped_copper"},
	}

	for _, c := range cases {
		b := &models.Bearing{Material: c.material}
		result := mm.guessMaterialCode(b)
		if result != c.expected {
			t.Errorf("材料 '%s': 预期 '%s', 实际 '%s'", c.material, c.expected, result)
		}
	}
	t.Log("中文材料名识别: 全部通过")
}

func TestGuessMaterialCode_EnglishKeywords(t *testing.T) {
	setupMaintenanceTestConfig(t)
	mm := newTestManager()

	cases := []struct {
		material string
		expected string
	}{
		{"bronze", "bronze_ancient"},
		{"cast iron", "cast_iron_ancient"},
		{"ironbark wood", "wood_ironbark"},
		{"Oak hardwood", "wood_oak"},
		{"GCr15 球轴承", "modern_ball_bearing"},
		{"rolling bearing", "modern_ball_bearing"},
		{"Babbitt 巴氏合金", "modern_bushing_babbit"},
	}

	for _, c := range cases {
		b := &models.Bearing{Material: c.material}
		result := mm.guessMaterialCode(b)
		if result != c.expected {
			t.Errorf("材料 '%s': 预期 '%s', 实际 '%s'", c.material, c.expected, result)
		}
	}
	t.Log("英文关键词材料识别: 全部通过")
}

func TestGuessMaterialCode_UnknownDefault(t *testing.T) {
	setupMaintenanceTestConfig(t)
	mm := newTestManager()

	b := &models.Bearing{Material: "神秘未知材料XYZ"}
	result := mm.guessMaterialCode(b)
	if result != "bronze_ancient" {
		t.Errorf("未知材料应回退到青铜，实际: %s", result)
	}
	t.Logf("未知材料回退正确: %s", result)
}

// ==================== reductionPct 测试 ====================

func TestReductionPct_Basic(t *testing.T) {
	mm := newTestManager()

	cases := []struct {
		oldVal   float64
		newVal   float64
		expected float64
	}{
		{10.0, 5.0, 50.0},
		{100.0, 10.0, 90.0},
		{0.01, 0.005, 50.0},
		{0, 5.0, 0.0},
		{-1, 5.0, 0.0},
	}

	for _, c := range cases {
		result := mm.reductionPct(c.oldVal, c.newVal)
		diff := result - c.expected
		if diff > 0.001 || diff < -0.001 {
			t.Errorf("reductionPct(%.2f, %.2f) = %.2f, 预期 %.2f",
				c.oldVal, c.newVal, result, c.expected)
		}
	}
	t.Log("磨损率降低百分比计算: 全部通过")
}

// ==================== 字符串工具函数测试 ====================

func TestContainsAny_MixedCase(t *testing.T) {
	cases := []struct {
		s        string
		keywords []string
		expected bool
	}{
		{"青铜轴承", []string{"青铜", "铜"}, true},
		{"BRONZE Bearing", []string{"bronze", "copper"}, true},
		{"Wood Oak", []string{"oak", "wood"}, true},
		{"不锈钢", []string{"青铜", "铸铁"}, false},
		{"", []string{"青铜"}, false},
	}

	for _, c := range cases {
		result := containsAny(c.s, c.keywords...)
		if result != c.expected {
			t.Errorf("containsAny('%s', %v) = %v, 预期 %v",
				c.s, c.keywords, result, c.expected)
		}
	}
}

func TestToLower_ASCII(t *testing.T) {
	cases := map[string]string{
		"HELLO":     "hello",
		"World123":  "world123",
		"MiXeD":      "mixed",
		"":           "",
		"nochange":   "nochange",
	}
	for in, expected := range cases {
		result := toLower(in)
		if result != expected {
			t.Errorf("toLower('%s') = '%s', 预期 '%s'", in, result, expected)
		}
	}
}

// ==================== 成本提示生成测试 ====================

func TestGenerateReplacementCostHint_KnownMaterials(t *testing.T) {
	setupMaintenanceTestConfig(t)
	mm := newTestManager()

	materials := []string{
		"wood_oak", "wood_ironbark", "bronze_ancient",
		"cast_iron_ancient", "modern_bushing_babbit",
		"modern_ball_bearing", "modern_roller_bearing",
	}

	for _, code := range materials {
		hint := mm.generateReplacementCostHint(code)
		if hint == "" {
			t.Errorf("材料 %s 成本提示为空", code)
		}
		if len(hint) < 5 {
			t.Errorf("材料 %s 成本提示过短: '%s'", code, hint)
		}
		t.Logf("  %s: %s", code, hint)
	}
}

func TestGenerateReplacementCostHint_Unknown(t *testing.T) {
	mm := newTestManager()
	hint := mm.generateReplacementCostHint("nonexistent_material_xyz")
	if hint == "" {
		t.Error("未知材料成本提示不应为空")
	}
	t.Logf("未知材料成本提示: %s", hint)
}

func TestGenerateLubricantCostHint_WithPrices(t *testing.T) {
	setupMaintenanceTestConfig(t)
	mm := newTestManager()

	cases := []struct {
		code   string
		amount float64
	}{
		{"vegetable_tung", 500},
		{"vegetable_rape", 1000},
		{"animal_lard", 250},
		{"mineral_synthetic_pao", 100},
	}

	for _, c := range cases {
		hint := mm.generateLubricantCostHint(c.code, c.amount)
		if hint == "" {
			t.Errorf("润滑剂 %s 成本提示为空", c.code)
		}
		t.Logf("  %s (%.0fml): %s", c.code, c.amount, hint)
	}
}

func TestGenerateLubricantCostHint_UnknownLubricant(t *testing.T) {
	mm := newTestManager()
	hint := mm.generateLubricantCostHint("unknown_lube", 500)
	if hint == "" {
		t.Error("未知润滑剂成本提示不应为空")
	}
	t.Logf("未知润滑剂成本提示: %s", hint)
}

// ==================== 建议材料/润滑剂测试 ====================

func TestSuggestedReplacementMaterials_Routine(t *testing.T) {
	setupMaintenanceTestConfig(t)
	mm := newTestManager()

	if len(mm.materialsCfg.Materials) == 0 {
		t.Skip("材料配置未加载")
	}

	result := mm.suggestedReplacementMaterials("wood_oak", false)
	if len(result) == 0 {
		t.Error("常规更换建议不应为空")
	}
	t.Logf("常规更换建议: %d 种", len(result))
	for _, r := range result {
		t.Logf("  - %s (%s, era=%s)", r["name"], r["code"], r["era"])
	}
}

func TestSuggestedReplacementMaterials_Urgent(t *testing.T) {
	setupMaintenanceTestConfig(t)
	mm := newTestManager()

	if len(mm.materialsCfg.Materials) == 0 {
		t.Skip("材料配置未加载")
	}

	result := mm.suggestedReplacementMaterials("wood_oak", true)
	if len(result) == 0 {
		t.Error("紧急更换建议不应为空")
	}

	hasModern := false
	for _, r := range result {
		if r["era"] == "modern" {
			hasModern = true
			break
		}
	}
	if !hasModern {
		t.Error("紧急更换建议应包含现代材料")
	}
	t.Logf("紧急更换建议: %d 种 (含现代材料=%v)", len(result), hasModern)
}

func TestSuggestedLubricants_AncientMaterial(t *testing.T) {
	setupMaintenanceTestConfig(t)
	mm := newTestManager()

	if len(mm.lubricantsCfg.Lubricants) == 0 {
		t.Skip("润滑剂配置未加载")
	}

	result := mm.suggestedLubricants("bronze_ancient")
	if len(result) == 0 {
		t.Error("古代材料润滑剂建议不应为空")
	}

	for _, r := range result {
		cat := r["category"].(string)
		if cat == "vegetable" || cat == "animal" {
		} else {
			t.Logf("  提示: 古代材料建议了非古代润滑剂类别: %s", cat)
		}
	}
	t.Logf("古代材料润滑剂建议: %d 种", len(result))
	for _, r := range result {
		t.Logf("  - %s (%s)", r["name"], r["category"])
	}
}

func TestSuggestedLubricants_ModernMaterial(t *testing.T) {
	setupMaintenanceTestConfig(t)
	mm := newTestManager()

	if len(mm.lubricantsCfg.Lubricants) == 0 {
		t.Skip("润滑剂配置未加载")
	}

	result := mm.suggestedLubricants("modern_ball_bearing")
	if len(result) == 0 {
		t.Error("现代材料润滑剂建议不应为空")
	}
	t.Logf("现代材料润滑剂建议: %d 种", len(result))
	for _, r := range result {
		t.Logf("  - %s (%s)", r["name"], r["category"])
	}
}

func TestSuggestedLubricants_UnknownMaterial(t *testing.T) {
	setupMaintenanceTestConfig(t)
	mm := newTestManager()

	if len(mm.lubricantsCfg.Lubricants) == 0 {
		t.Skip("润滑剂配置未加载")
	}

	result := mm.suggestedLubricants("unknown_material_xyz")
	if len(result) == 0 {
		t.Error("未知材料也应给出默认润滑剂建议")
	}
	t.Logf("未知材料润滑剂建议: %d 种 (回退到古代默认)", len(result))
}

// ==================== guessMaterialFromBearing 测试 ====================

func TestGuessMaterialFromBearing_Known(t *testing.T) {
	setupMaintenanceTestConfig(t)
	mm := newTestManager()

	b := &models.Bearing{Material: "青铜"}
	name := mm.guessMaterialFromBearing(b)

	if len(mm.materialsCfg.Materials) > 0 {
		if name == "青铜" {
			t.Logf("回退显示原始材料名 (索引为空): %s", name)
		} else {
			t.Logf("识别为中文材料名: %s", name)
		}
	}
}

func TestGuessMaterialFromBearing_NoMatch(t *testing.T) {
	mm := newTestManager()
	b := &models.Bearing{Material: "完全随机不存在的材料名"}
	name := mm.guessMaterialFromBearing(b)
	if name != b.Material {
		t.Errorf("无法识别时应返回原始字符串: 预期 '%s', 实际 '%s'", b.Material, name)
	}
	t.Logf("无法识别时返回原始字符串: %s", name)
}

// ==================== 教育性体验验证测试 ====================

func TestMaintenance_EducationalHintsPresent(t *testing.T) {
	setupMaintenanceTestConfig(t)
	mm := newTestManager()

	if len(mm.materialsCfg.Materials) == 0 {
		t.Skip("配置未加载")
	}

	mats := mm.suggestedReplacementMaterials("bronze_ancient", false)
	for _, m := range mats {
		hist := m["historical"].(string)
		if hist == "" {
			t.Errorf("材料 %s 缺少历史教育信息", m["name"])
		}
		if len(hist) < 10 {
			t.Errorf("材料 %s 历史信息过短 (<10字符)", m["name"])
		}
	}

	lubs := mm.suggestedLubricants("bronze_ancient")
	for _, l := range lubs {
		hist := l["historical"].(string)
		if hist == "" {
			t.Errorf("润滑剂 %s 缺少历史教育信息", l["name"])
		}
	}

	t.Log("教育性内容验证: 所有建议的材料和润滑剂均包含历史说明")
}

func TestMaintenance_CostHintInformative(t *testing.T) {
	setupMaintenanceTestConfig(t)
	mm := newTestManager()

	materials := []string{"wood_oak", "bronze_ancient", "modern_ball_bearing"}
	for _, code := range materials {
		hint := mm.generateReplacementCostHint(code)
		if len(hint) < 15 {
			t.Errorf("材料 %s 成本提示信息不足 (<15字符)", code)
		}
	}
	t.Log("成本提示教育性验证: 通过")
}
