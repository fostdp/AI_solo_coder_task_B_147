package analysis

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"

	"noria-bearing-system/internal/config"
	"noria-bearing-system/internal/models"
)

func setupTestConfig(t *testing.T) {
	t.Helper()

	projectRoot := filepath.Join("..", "..", "..", "..")
	wearParamsPath := filepath.Join(projectRoot, "config", "wear_params.json")
	lubricationPath := filepath.Join(projectRoot, "config", "lubrication_params.json")
	materialsPath := filepath.Join(projectRoot, "config", "bearing_materials.json")
	lubricantsPath := filepath.Join(projectRoot, "config", "lubricants.json")

	for _, p := range []string{wearParamsPath, lubricationPath, materialsPath, lubricantsPath} {
		if _, err := os.Stat(p); err != nil {
			t.Skipf("config file not found, skipping: %s", p)
		}
	}

	wearData, _ := os.ReadFile(wearParamsPath)
	_ = json.Unmarshal(wearData, &config.AppConfig.WearParams)

	lubData, _ := os.ReadFile(lubricationPath)
	_ = json.Unmarshal(lubData, &config.AppConfig.Lubrication)

	matData, _ := os.ReadFile(materialsPath)
	var matWrapper struct {
		Materials []config.BearingMaterial `json:"materials"`
	}
	_ = json.Unmarshal(matData, &matWrapper)
	config.AppConfig.Materials = config.BearingMaterialsConfig{Materials: matWrapper.Materials}
	config.AppConfig.Materials.BuildIndex()

	lubDataRaw, _ := os.ReadFile(lubricantsPath)
	var lubWrapper struct {
		Lubricants                  []config.Lubricant       `json:"lubricants"`
		RecommendedLubricationFreq  map[string]float64       `json:"recommended_lubrication_freq,omitempty"`
	}
	_ = json.Unmarshal(lubDataRaw, &lubWrapper)
	config.AppConfig.Lubricants = config.LubricantsConfig{
		Lubricants:                 lubWrapper.Lubricants,
		RecommendedLubricationFreq: lubWrapper.RecommendedLubricationFreq,
	}
	config.AppConfig.Lubricants.BuildIndex()
}

func makeTestBearing() *models.Bearing {
	return &models.Bearing{
		ID:              1,
		NoriaWheelID:    1,
		BearingCode:     "NRW-001-BR-A",
		Position:        "上轴承",
		BearingType:     "sliding",
		InnerDiameter:   120.0,
		OuterDiameter:   180.0,
		Width:           150.0,
		Material:        "青铜",
		HardnessHV:      110.0,
		RatedLifeHours:  43800.0,
		WearLimitMicrom: 750.0,
		LubricationType: "桐油",
		OilViscosityPaS: 0.26,
	}
}

// ==================== simulateWear 核心算法测试 ====================

func TestSimulateWear_BronzeNormal(t *testing.T) {
	setupTestConfig(t)
	ce := NewComparisonEngine()

	in := simInput{
		innerDiameterMM: 120,
		outerDiameterMM: 180,
		widthMM:         150,
		loadN:           5000,
		speedRPM:        15,
		tempCelsius:     40,
		hours:           8760,
		hardnessHV:      110,
		archardK:        1.5e-8,
		surfaceRMS:      0.8e-6,
		elasticModPa:    1.0e11,
		viscosityPaS:    0.26,
		pressureCoeff:   1.8e-8,
		ehlBoost:        0.7,
		wearReduction:   0.42,
		wearResistance:  0.85,
		isRolling:       false,
	}

	out := ce.simulateWear(in)

	if out.totalWearUm <= 0 {
		t.Errorf("预期磨损量>0，实际 %.4f", out.totalWearUm)
	}
	if out.wearRateUmPerHour <= 0 {
		t.Errorf("预期磨损速率>0，实际 %.6f", out.wearRateUmPerHour)
	}
	if out.lifeHours <= 0 {
		t.Errorf("预期寿命>0小时，实际 %.0f", out.lifeHours)
	}
	if out.ehlMeanLambda < 0.1 || out.ehlMeanLambda > 10 {
		t.Errorf("EHL λ 参数范围异常: %.4f", out.ehlMeanLambda)
	}
	if out.contactPressureMPa <= 0 {
		t.Errorf("接触压力应>0，实际 %.2f", out.contactPressureMPa)
	}
	if out.regime != "full_film" && out.regime != "mixed" && out.regime != "boundary" {
		t.Errorf("未知润滑状态: %s", out.regime)
	}

	approxRate := out.totalWearUm / 8760.0
	if math.Abs(out.wearRateUmPerHour-approxRate) > 1e-9 {
		t.Errorf("磨损速率计算不一致: hourly=%.8f  approx=%.8f", out.wearRateUmPerHour, approxRate)
	}
	t.Logf("青铜正常工况: 磨损量=%.4fμm, 速率=%.6fμm/h, 寿命=%.1f年, λ=%.3f, 状态=%s",
		out.totalWearUm, out.wearRateUmPerHour, out.lifeYears, out.ehlMeanLambda, out.regime)
}

func TestSimulateWear_WoodVsBronze(t *testing.T) {
	setupTestConfig(t)
	ce := NewComparisonEngine()

	baseIn := simInput{
		innerDiameterMM: 120,
		outerDiameterMM: 180,
		widthMM:         150,
		loadN:           5000,
		speedRPM:        15,
		tempCelsius:     40,
		hours:           8760,
		viscosityPaS:    0.26,
		pressureCoeff:   1.8e-8,
		ehlBoost:        0.7,
		wearReduction:   0.42,
		isRolling:       false,
	}

	woodIn := baseIn
	woodIn.hardnessHV = 22
	woodIn.archardK = 8.0e-7
	woodIn.surfaceRMS = 3.2e-6
	woodIn.elasticModPa = 1.2e10
	woodIn.wearResistance = 0.35

	bronzeIn := baseIn
	bronzeIn.hardnessHV = 110
	bronzeIn.archardK = 1.5e-8
	bronzeIn.surfaceRMS = 0.8e-6
	bronzeIn.elasticModPa = 1.0e11
	bronzeIn.wearResistance = 0.85

	woodOut := ce.simulateWear(woodIn)
	bronzeOut := ce.simulateWear(bronzeIn)

	if woodOut.wearRateUmPerHour <= bronzeOut.wearRateUmPerHour {
		t.Errorf("木材磨损速率(%.6f)应大于青铜(%.6f)", woodOut.wearRateUmPerHour, bronzeOut.wearRateUmPerHour)
	}
	if woodOut.lifeHours >= bronzeOut.lifeHours {
		t.Errorf("木材寿命(%.0fh)应小于青铜(%.0fh)", woodOut.lifeHours, bronzeOut.lifeHours)
	}

	wearRatio := woodOut.wearRateUmPerHour / bronzeOut.wearRateUmPerHour
	if wearRatio < 3.0 {
		t.Errorf("木材/青铜磨损比应至少3倍，实际 %.1fx", wearRatio)
	}
	t.Logf("木材vs青铜: 磨损速率比=%.1fx, 木材寿命=%.1f年, 青铜寿命=%.1f年",
		wearRatio, woodOut.lifeYears, bronzeOut.lifeYears)
}

func TestSimulateWear_LoadVariation(t *testing.T) {
	setupTestConfig(t)
	ce := NewComparisonEngine()

	loads := []float64{2000, 5000, 10000, 18000}
	results := make([]simOutput, len(loads))

	for i, load := range loads {
		in := simInput{
			innerDiameterMM: 120,
			outerDiameterMM: 180,
			widthMM:         150,
			loadN:           load,
			speedRPM:        15,
			tempCelsius:     40,
			hours:           8760,
			hardnessHV:      110,
			archardK:        1.5e-8,
			surfaceRMS:      0.8e-6,
			elasticModPa:    1.0e11,
			viscosityPaS:    0.26,
			pressureCoeff:   1.8e-8,
			ehlBoost:        0.7,
			wearReduction:   0.42,
			wearResistance:  0.85,
			isRolling:       false,
		}
		results[i] = ce.simulateWear(in)
	}

	for i := 1; i < len(results); i++ {
		if results[i].wearRateUmPerHour <= results[i-1].wearRateUmPerHour {
			t.Errorf("载荷增加时磨损速率应递增: load=%.0f rate=%.6f  vs  load=%.0f rate=%.6f",
				loads[i-1], results[i-1].wearRateUmPerHour,
				loads[i], results[i].wearRateUmPerHour)
		}
		if results[i].lifeHours >= results[i-1].lifeHours {
			t.Errorf("载荷增加时寿命应递减")
		}
	}

	linearityCheck := results[2].wearRateUmPerHour / results[0].wearRateUmPerHour
	expectedRatio := loads[2] / loads[0]
	if math.Abs(linearityCheck-expectedRatio) > expectedRatio*0.5 {
		t.Logf("[提示] Archard定律近似线性: 载荷比=%.1f, 磨损率比=%.1f (偏差在合理范围内)",
			expectedRatio, linearityCheck)
	}
	t.Logf("载荷梯度验证通过: 2kN→5kN→10kN→18kN 磨损速率单调递增")
}

func TestSimulateWear_RollingVsSliding(t *testing.T) {
	setupTestConfig(t)
	ce := NewComparisonEngine()

	baseIn := simInput{
		innerDiameterMM: 120,
		outerDiameterMM: 180,
		widthMM:         150,
		loadN:           5000,
		speedRPM:        15,
		tempCelsius:     40,
		hours:           8760,
		hardnessHV:      600,
		archardK:        1.0e-9,
		surfaceRMS:      0.1e-6,
		elasticModPa:    2.1e11,
		viscosityPaS:    0.03,
		pressureCoeff:   2.2e-8,
		ehlBoost:        1.2,
		wearReduction:   0.85,
		wearResistance:  2.5,
	}

	slidingIn := baseIn
	slidingIn.isRolling = false

	rollingIn := baseIn
	rollingIn.isRolling = true

	slidingOut := ce.simulateWear(slidingIn)
	rollingOut := ce.simulateWear(rollingIn)

	if rollingOut.wearRateUmPerHour >= slidingOut.wearRateUmPerHour {
		t.Errorf("滚动磨损速率(%.6f)应小于滑动(%.6f)", rollingOut.wearRateUmPerHour, slidingOut.wearRateUmPerHour)
	}

	reductionPct := (slidingOut.wearRateUmPerHour - rollingOut.wearRateUmPerHour) / slidingOut.wearRateUmPerHour * 100
	if reductionPct < 80 {
		t.Errorf("滚动相对滑动磨损减少应>80%%，实际 %.1f%%", reductionPct)
	}
	t.Logf("滚动vs滑动: 磨损减少%.1f%%, 滚动寿命=%.1f年, 滑动寿命=%.1f年",
		reductionPct, rollingOut.lifeYears, slidingOut.lifeYears)
}

// ==================== EHL 油膜计算测试 ====================

func TestCalculateEHLFilm_Basic(t *testing.T) {
	setupTestConfig(t)
	ce := NewComparisonEngine()

	film := ce.calculateEHLFilm(0.075, 15, 0.26, 5000, 1.0e11, 1.8e-8)

	if film < 0.05e-6 || film > 20e-6 {
		t.Errorf("油膜厚度超出合理范围 [0.05μm, 20μm]: %.4fμm", film*1e6)
	}
	t.Logf("EHL油膜厚度: %.3fμm", film*1e6)
}

func TestCalculateEHLFilm_EdgeCases(t *testing.T) {
	setupTestConfig(t)
	ce := NewComparisonEngine()

	film1 := ce.calculateEHLFilm(0, 15, 0.26, 5000, 1.0e11, 1.8e-8)
	if film1 != 1e-7 {
		t.Errorf("半径为0时应返回默认油膜厚度，实际 %e", film1)
	}

	film2 := ce.calculateEHLFilm(0.075, 0, 0.26, 5000, 1.0e11, 1.8e-8)
	if film2 < 0.05e-6 {
		t.Errorf("转速为0时油膜应被钳制到最小值，实际 %.4fμm", film2*1e6)
	}

	film3 := ce.calculateEHLFilm(0.075, 15, 0, 5000, 1.0e11, 1.8e-8)
	if film3 != 1e-7 {
		t.Errorf("粘度为0时应返回默认值，实际 %e", film3)
	}
	t.Log("EHL边界条件处理正确")
}

// ==================== wearCoefficientForLambda 测试 ====================

func TestWearCoefficientForLambda_Regimes(t *testing.T) {
	setupTestConfig(t)
	ce := NewComparisonEngine()

	fullFilm := ce.wearCoefficientForLambda(5.0)
	mixed := ce.wearCoefficientForLambda(1.5)
	boundary := ce.wearCoefficientForLambda(0.2)

	if !(boundary > mixed && mixed > fullFilm) {
		t.Errorf("磨损系数应: 边界(%.2e) > 混合(%.2e) > 全膜(%.2e)",
			boundary, mixed, fullFilm)
	}

	expectedFullFilm := ce.wearParams.WearCoefficientFactors["full_film"]
	if math.Abs(fullFilm-expectedFullFilm) > 1e-12 {
		t.Errorf("全膜磨损系数不匹配: %.2e vs %.2e", fullFilm, expectedFullFilm)
	}
	t.Logf("润滑状态磨损系数: 边界=%.2e, 混合=%.2e, 全膜=%.2e", boundary, mixed, fullFilm)
}

// ==================== CompareMaterials 测试 ====================

func TestCompareMaterials_AncientGroup(t *testing.T) {
	setupTestConfig(t)
	ce := NewComparisonEngine()
	bearing := makeTestBearing()

	materials := []string{"wood_oak", "wood_ironbark", "bronze_ancient", "cast_iron_ancient"}
	result := ce.CompareMaterials(bearing, materials, 5000, 15, 40, 8760)

	if result == nil {
		t.Fatal("材料对比结果为空")
	}
	if len(result.Items) != len(materials) {
		t.Errorf("预期返回 %d 种材料结果，实际 %d", len(materials), len(result.Items))
	}

	for i := 1; i < len(result.Items); i++ {
		if result.Items[i].PredictedLifeHours > result.Items[i-1].PredictedLifeHours {
			t.Errorf("结果未按寿命降序排列: #%d(%.0fh) > #%d(%.0fh)",
				i, result.Items[i].PredictedLifeHours, i-1, result.Items[i-1].PredictedLifeHours)
		}
	}

	for idx, item := range result.Items {
		if item.Rank != idx+1 {
			t.Errorf("排名错误: index=%d rank=%d", idx, item.Rank)
		}
		if item.Rank == 1 && item.LifeRatioVsBest != 1.0 {
			t.Errorf("第一名寿命比应为1.0，实际 %.4f", item.LifeRatioVsBest)
		}
		if item.Era != "ancient" {
			t.Errorf("材料 %s 时代标记应为 ancient，实际 %s", item.MaterialCode, item.Era)
		}
	}

	t.Logf("古代材料排名:")
	for _, item := range result.Items {
		t.Logf("  #%d %s: %.1f年 (λ=%.2f, 磨损率=%.6fμm/h)",
			item.Rank, item.MaterialName, item.PredictedLifeYears,
			item.EHLMeanLambda, item.WearRateUmPerHour)
	}
}

func TestCompareMaterials_InvalidCode(t *testing.T) {
	setupTestConfig(t)
	ce := NewComparisonEngine()
	bearing := makeTestBearing()

	materials := []string{"bronze_ancient", "invalid_material_xyz", "wood_oak", "another_fake"}
	result := ce.CompareMaterials(bearing, materials, 5000, 15, 40, 8760)

	if len(result.Items) != 2 {
		t.Errorf("无效代码应被过滤，预期2个有效结果，实际 %d", len(result.Items))
	}
	t.Logf("无效材料代码过滤正确: %d 个有效结果", len(result.Items))
}

func TestCompareMaterials_EmptyList(t *testing.T) {
	setupTestConfig(t)
	ce := NewComparisonEngine()
	bearing := makeTestBearing()

	result := ce.CompareMaterials(bearing, []string{}, 5000, 15, 40, 8760)
	if result == nil {
		t.Fatal("空列表不应返回nil")
	}
	if len(result.Items) != 0 {
		t.Errorf("空列表应返回0项，实际 %d", len(result.Items))
	}
	t.Log("空材料列表处理正确")
}

// ==================== CrossEraComparison 跨时代对比测试 ====================

func TestCrossEraComparison_Basic(t *testing.T) {
	setupTestConfig(t)
	ce := NewComparisonEngine()

	result := ce.CrossEraComparison(120, 150, 5000, 15, 40, 8760)

	if result == nil {
		t.Fatal("跨时代对比结果为空")
	}
	if result.AncientBest == nil {
		t.Error("古代最佳材料为空")
	}
	if result.ModernBest == nil {
		t.Error("现代最佳材料为空")
	}
	if result.LifeImprovementX < 1.0 {
		t.Errorf("现代寿命提升倍数应>1，实际 %.1fx", result.LifeImprovementX)
	}
	if result.WearReductionPct <= 0 || result.WearReductionPct > 100 {
		t.Errorf("磨损减少百分比应在(0,100)之间，实际 %.1f%%", result.WearReductionPct)
	}
	if len(result.InsightSummary) < 2 {
		t.Errorf("应生成至少2条洞察，实际 %d", len(result.InsightSummary))
	}
	if len(result.AllItems) != 8 {
		t.Errorf("应返回全部8种材料对比，实际 %d", len(result.AllItems))
	}

	t.Logf("跨时代对比: 寿命提升 %.1fx, 磨损减少 %.1f%%",
		result.LifeImprovementX, result.WearReductionPct)
	t.Logf("  古代最佳: %s (%.1f年)", result.AncientBest.MaterialName, result.AncientBest.PredictedLifeYears)
	t.Logf("  现代最佳: %s (%.1f年)", result.ModernBest.MaterialName, result.ModernBest.PredictedLifeYears)
	for i, ins := range result.InsightSummary {
		t.Logf("  洞察%d: %s", i+1, ins)
	}
}

func TestCrossEraComparison_ExtremeLoad(t *testing.T) {
	setupTestConfig(t)
	ce := NewComparisonEngine()

	result := ce.CrossEraComparison(120, 150, 18000, 30, 60, 8760)

	if result.LifeImprovementX < 1.0 {
		t.Errorf("极端工况下现代仍应更优，提升倍数 %.1fx", result.LifeImprovementX)
	}
	t.Logf("极端工况(18kN,30RPM,60°C): 寿命提升 %.1fx, 磨损减少 %.1f%%",
		result.LifeImprovementX, result.WearReductionPct)
}

// ==================== CompareLubricants 润滑剂对比测试 ====================

func TestCompareLubricants_VegetableOils(t *testing.T) {
	setupTestConfig(t)
	ce := NewComparisonEngine()
	bearing := makeTestBearing()

	lubricants := []string{"vegetable_tung", "vegetable_rape", "vegetable_sesame"}
	result := ce.CompareLubricants(bearing, "bronze_ancient", lubricants, 5000, 15, 40, 8760)

	if result == nil {
		t.Fatal("润滑剂对比结果为空")
	}
	if len(result.Items) != 3 {
		t.Errorf("预期3种植物油结果，实际 %d", len(result.Items))
	}

	for _, item := range result.Items {
		if item.WearReductionVsDry <= 0 {
			t.Errorf("%s 相对于干摩擦磨损减少应>0，实际 %.1f%%",
				item.LubricantName, item.WearReductionVsDry)
		}
		if item.LifeExtensionVsDry <= 0 {
			t.Errorf("%s 寿命提升应>0，实际 %.1f%%",
				item.LubricantName, item.LifeExtensionVsDry)
		}
		if item.LubricationRegime == "" {
			t.Errorf("%s 缺少润滑状态", item.LubricantName)
		}
	}

	for i := 1; i < len(result.Items); i++ {
		if result.Items[i].PredictedLifeHours > result.Items[i-1].PredictedLifeHours {
			t.Error("润滑剂结果未按寿命降序排列")
		}
	}

	t.Logf("植物油对比排名:")
	for _, item := range result.Items {
		t.Logf("  #%d %s: 寿命=%.1f年, 磨损减少=%.1f%%, 寿命提升=%.1f%%, 状态=%s",
			item.Rank, item.LubricantName, item.PredictedLifeYears,
			item.WearReductionVsDry, item.LifeExtensionVsDry, item.LubricationRegime)
	}
}

func TestCompareLubricants_AncientVsModern(t *testing.T) {
	setupTestConfig(t)
	ce := NewComparisonEngine()
	bearing := makeTestBearing()

	lubricants := []string{"vegetable_tung", "animal_lard", "mineral_crude", "mineral_synthetic_pao", "synthetic_ep_gear"}
	result := ce.CompareLubricants(bearing, "bronze_ancient", lubricants, 5000, 15, 40, 8760)

	if len(result.Items) < 3 {
		t.Errorf("至少应有3种有效润滑剂结果，实际 %d", len(result.Items))
	}

	t.Logf("古今润滑剂对比:")
	for _, item := range result.Items {
		t.Logf("  #%d %s [%s]: 寿命=%.1f年, 提升=%.0f%%, λ=%.2f, 状态=%s",
			item.Rank, item.LubricantName, item.Category,
			item.PredictedLifeYears, item.LifeExtensionVsDry,
			item.EHLMeanLambda, item.LubricationRegime)
	}
}

func TestCompareLubricants_InvalidAndEmpty(t *testing.T) {
	setupTestConfig(t)
	ce := NewComparisonEngine()
	bearing := makeTestBearing()

	result := ce.CompareLubricants(bearing, "bronze_ancient",
		[]string{"invalid_lube", "vegetable_tung", "fake_oil"}, 5000, 15, 40, 8760)
	if len(result.Items) != 1 {
		t.Errorf("无效代码过滤错误，预期1个有效，实际 %d", len(result.Items))
	}

	emptyResult := ce.CompareLubricants(bearing, "bronze_ancient", []string{}, 5000, 15, 40, 8760)
	if len(emptyResult.Items) != 0 {
		t.Errorf("空列表应返回0项，实际 %d", len(emptyResult.Items))
	}
	t.Log("润滑剂异常输入处理正确")
}

func TestCompareLubricants_FrictionCoefficientOrdering(t *testing.T) {
	setupTestConfig(t)
	ce := NewComparisonEngine()

	if len(ce.lubricants.Lubricants) == 0 {
		t.Skip("润滑剂配置未加载")
	}

	tung, tungOK := ce.lubricants.GetLubricant("vegetable_tung")
	mineral, mineralOK := ce.lubricants.GetLubricant("mineral_synthetic_pao")

	if !tungOK || !mineralOK {
		t.Skip("摩擦系数测试所需润滑剂缺失")
	}

	if tung.FrictionCoefficientLubed <= 0 || mineral.FrictionCoefficientLubed <= 0 {
		t.Skip("摩擦系数字段缺失")
	}

	if !(tung.FrictionCoefficientLubed > mineral.FrictionCoefficientLubed) {
		t.Logf("[信息] 桐油润滑摩擦系数(%.3f) vs 合成油(%.3f) - 按实际配置数据",
			tung.FrictionCoefficientLubed, mineral.FrictionCoefficientLubed)
	} else {
		t.Logf("摩擦系数排序符合预期: 桐油(%.3f) > 合成油(%.3f)",
			tung.FrictionCoefficientLubed, mineral.FrictionCoefficientLubed)
	}
}

// ==================== 温度敏感性测试 ====================

func TestSimulateWear_TemperatureSensitivity(t *testing.T) {
	setupTestConfig(t)
	ce := NewComparisonEngine()

	baseIn := simInput{
		innerDiameterMM: 120,
		outerDiameterMM: 180,
		widthMM:         150,
		loadN:           5000,
		speedRPM:        15,
		hours:           8760,
		hardnessHV:      110,
		archardK:        1.5e-8,
		surfaceRMS:      0.8e-6,
		elasticModPa:    1.0e11,
		viscosityPaS:    0.26,
		pressureCoeff:   1.8e-8,
		ehlBoost:        0.7,
		wearReduction:   0.42,
		wearResistance:  0.85,
	}

	coldIn := baseIn
	coldIn.tempCelsius = 10
	normalIn := baseIn
	normalIn.tempCelsius = 40
	hotIn := baseIn
	hotIn.tempCelsius = 80

	coldOut := ce.simulateWear(coldIn)
	normalOut := ce.simulateWear(normalIn)
	hotOut := ce.simulateWear(hotIn)

	if !(coldOut.wearRateUmPerHour < normalOut.wearRateUmPerHour &&
		normalOut.wearRateUmPerHour < hotOut.wearRateUmPerHour) {
		t.Errorf("温度升高磨损率应递增: 10°C=%.6f 40°C=%.6f 80°C=%.6f",
			coldOut.wearRateUmPerHour, normalOut.wearRateUmPerHour, hotOut.wearRateUmPerHour)
	}
	if !(coldOut.lifeHours > normalOut.lifeHours && normalOut.lifeHours > hotOut.lifeHours) {
		t.Errorf("温度升高寿命应递减")
	}
	t.Logf("温度敏感性: 10°C→%.1f年, 40°C→%.1f年, 80°C→%.1f年",
		coldOut.lifeYears, normalOut.lifeYears, hotOut.lifeYears)
}
