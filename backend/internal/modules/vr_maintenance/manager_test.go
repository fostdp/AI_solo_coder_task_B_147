package vr_maintenance

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"noria-bearing-system/internal/config"
	"noria-bearing-system/internal/models"
)

func setupVRTestConfig(t *testing.T) {
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
		Lubricants                 []config.Lubricant `json:"lubricants"`
		RecommendedLubricationFreq map[string]float64 `json:"recommended_lubrication_freq,omitempty"`
	}
	_ = json.Unmarshal(lubDataRaw, &lubWrapper)
	config.AppConfig.Lubricants = config.LubricantsConfig{
		Lubricants:                 lubWrapper.Lubricants,
		RecommendedLubricationFreq: lubWrapper.RecommendedLubricationFreq,
	}
	config.AppConfig.Lubricants.BuildIndex()

	config.AppConfig.WearParams = config.WearParamsConfig{
		PressureViscosityCoefficient: 2.2e-8,
	}
}

func newTestVRManager() *VRMaintenanceManager {
	return &VRMaintenanceManager{
		db:             nil,
		mc:             nil,
		la:             nil,
		materialsCfg:   &config.AppConfig.Materials,
		lubricantsCfg:  &config.AppConfig.Lubricants,
		alertThreshold: 0.7,
	}
}

func TestGuessMaterialCode_AncientChinese(t *testing.T) {
	setupVRTestConfig(t)
	vrm := newTestVRManager()

	cases := []struct {
		material string
		expected string
	}{
		{"青铜", "bronze_ancient"},
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
		result := vrm.guessMaterialCode(b)
		if result != c.expected {
			t.Errorf("材料 '%s': 预期 '%s', 实际 '%s'", c.material, c.expected, result)
		}
	}
}

func TestGuessMaterialCode_EnglishKeywords(t *testing.T) {
	setupVRTestConfig(t)
	vrm := newTestVRManager()

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
		{"Babbitt 巴氏合金", "modern_bushing_babit"},
	}

	for _, c := range cases {
		b := &models.Bearing{Material: c.material}
		result := vrm.guessMaterialCode(b)
		if result != c.expected {
			t.Errorf("材料 '%s': 预期 '%s', 实际 '%s'", c.material, c.expected, result)
		}
	}
}

func TestGuessMaterialCode_UnknownDefault(t *testing.T) {
	setupVRTestConfig(t)
	vrm := newTestVRManager()

	b := &models.Bearing{Material: "神秘未知材料XYZ"}
	result := vrm.guessMaterialCode(b)
	if result != "bronze_ancient" {
		t.Errorf("未知材料应回退到青铜，实际: %s", result)
	}
}

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
			t.Errorf("containsAny('%s', %v) = %v, 预期 %v", c.s, c.keywords, result, c.expected)
		}
	}
}

func TestToLower_ASCII(t *testing.T) {
	cases := map[string]string{
		"HELLO":    "hello",
		"World123": "world123",
		"MiXeD":    "mixed",
		"":         "",
		"nochange": "nochange",
	}
	for in, expected := range cases {
		result := toLower(in)
		if result != expected {
			t.Errorf("toLower('%s') = '%s', 预期 '%s'", in, result, expected)
		}
	}
}

func TestReductionPct_Basic(t *testing.T) {
	vrm := newTestVRManager()

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
		result := vrm.reductionPct(c.oldVal, c.newVal)
		diff := result - c.expected
		if diff > 0.001 || diff < -0.001 {
			t.Errorf("reductionPct(%.2f, %.2f) = %.2f, 预期 %.2f",
				c.oldVal, c.newVal, result, c.expected)
		}
	}
}

func TestGenerateReplacementCostHint_KnownMaterials(t *testing.T) {
	setupVRTestConfig(t)
	vrm := newTestVRManager()

	materials := []string{
		"wood_oak", "wood_ironbark", "bronze_ancient",
		"cast_iron_ancient", "modern_bushing_babbit",
		"modern_ball_bearing", "modern_roller_bearing",
	}

	for _, code := range materials {
		hint := vrm.generateReplacementCostHint(code)
		if hint == "" {
			t.Errorf("材料 %s 成本提示为空", code)
		}
		if len(hint) < 5 {
			t.Errorf("材料 %s 成本提示过短: '%s'", code, hint)
		}
	}
}

func TestGenerateReplacementCostHint_Unknown(t *testing.T) {
	vrm := newTestVRManager()
	hint := vrm.generateReplacementCostHint("nonexistent_material_xyz")
	if hint == "" {
		t.Error("未知材料成本提示不应为空")
	}
}

func TestGenerateLubricantCostHint_WithPrices(t *testing.T) {
	setupVRTestConfig(t)
	vrm := newTestVRManager()

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
		hint := vrm.generateLubricantCostHint(c.code, c.amount)
		if hint == "" {
			t.Errorf("润滑剂 %s 成本提示为空", c.code)
		}
	}
}

func TestGenerateLubricantCostHint_UnknownLubricant(t *testing.T) {
	vrm := newTestVRManager()
	hint := vrm.generateLubricantCostHint("unknown_lube", 500)
	if hint == "" {
		t.Error("未知润滑剂成本提示不应为空")
	}
}

func TestSuggestedReplacementMaterials_Routine(t *testing.T) {
	setupVRTestConfig(t)
	vrm := newTestVRManager()

	if len(vrm.materialsCfg.Materials) == 0 {
		t.Skip("材料配置未加载")
	}

	result := vrm.suggestedReplacementMaterials("wood_oak", false)
	if len(result) == 0 {
		t.Error("常规更换建议不应为空")
	}
}

func TestSuggestedReplacementMaterials_Urgent(t *testing.T) {
	setupVRTestConfig(t)
	vrm := newTestVRManager()

	if len(vrm.materialsCfg.Materials) == 0 {
		t.Skip("材料配置未加载")
	}

	result := vrm.suggestedReplacementMaterials("wood_oak", true)
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
}

func TestSuggestedLubricants_AncientMaterial(t *testing.T) {
	setupVRTestConfig(t)
	vrm := newTestVRManager()

	if len(vrm.lubricantsCfg.Lubricants) == 0 {
		t.Skip("润滑剂配置未加载")
	}

	result := vrm.suggestedLubricants("bronze_ancient")
	if len(result) == 0 {
		t.Error("古代材料润滑剂建议不应为空")
	}
}

func TestSuggestedLubricants_ModernMaterial(t *testing.T) {
	setupVRTestConfig(t)
	vrm := newTestVRManager()

	if len(vrm.lubricantsCfg.Lubricants) == 0 {
		t.Skip("润滑剂配置未加载")
	}

	result := vrm.suggestedLubricants("modern_ball_bearing")
	if len(result) == 0 {
		t.Error("现代材料润滑剂建议不应为空")
	}
}

func TestGuessMaterialFromBearing_Known(t *testing.T) {
	setupVRTestConfig(t)
	vrm := newTestVRManager()

	b := &models.Bearing{Material: "青铜"}
	name := vrm.guessMaterialFromBearing(b)
	if name == "" {
		t.Error("guessMaterialFromBearing should return non-empty string")
	}
}

func TestGuessMaterialFromBearing_NoMatch(t *testing.T) {
	vrm := newTestVRManager()
	b := &models.Bearing{Material: "完全随机不存在的材料名"}
	name := vrm.guessMaterialFromBearing(b)
	if name != b.Material {
		t.Errorf("无法识别时应返回原始字符串: 预期 '%s', 实际 '%s'", b.Material, name)
	}
}

func TestVRMaintenance_EducationalHintsPresent(t *testing.T) {
	setupVRTestConfig(t)
	vrm := newTestVRManager()

	if len(vrm.materialsCfg.Materials) == 0 || len(vrm.lubricantsCfg.Lubricants) == 0 {
		t.Skip("配置未加载")
	}

	mats := vrm.suggestedReplacementMaterials("bronze_ancient", false)
	for _, m := range mats {
		hist := m["historical"].(string)
		if hist == "" {
			t.Errorf("材料 %s 缺少历史教育信息", m["name"])
		}
	}

	lubs := vrm.suggestedLubricants("bronze_ancient")
	for _, l := range lubs {
		hist := l["historical"].(string)
		if hist == "" {
			t.Errorf("润滑剂 %s 缺少历史教育信息", l["name"])
		}
	}
}
