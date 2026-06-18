package era_comparator

import (
	"fmt"
	"sort"
	"time"

	"noria-bearing-system/internal/config"
	"noria-bearing-system/internal/models"
	"noria-bearing-system/internal/modules/material_comparator"
	"noria-bearing-system/internal/modules/wearcalc"
)

type EraComparator struct {
	materials  *config.BearingMaterialsConfig
	lubricants *config.LubricantsConfig
	calculator *wearcalc.WearCalculator
	mc         *material_comparator.MaterialComparator
}

func NewEraComparator() *EraComparator {
	return &EraComparator{
		materials:  &config.AppConfig.Materials,
		lubricants: &config.AppConfig.Lubricants,
		calculator: wearcalc.NewWearCalculator(2),
		mc:         material_comparator.NewMaterialComparator(),
	}
}

func (ec *EraComparator) Close() {
	ec.calculator.Close()
	ec.mc.Close()
}

func (ec *EraComparator) CrossEraComparison(
	bearingDiameterMM, widthMM, loadN, speedRPM, tempCelsius, simHours float64,
) *models.CrossEraComparisonResult {
	ancientCodes := []string{"wood_oak", "wood_ironbark", "bronze_ancient", "cast_iron_ancient", "wood_wrapped_copper"}
	modernCodes := []string{"modern_bushing_babbit", "modern_ball_bearing", "modern_roller_bearing"}

	ancientLub := ec.getDefaultAncientLubricant()
	modernLub := ec.getDefaultModernLubricant()

	ancientResult := ec.mc.CompareMaterialsGeneric(
		bearingDiameterMM, bearingDiameterMM*1.5, widthMM,
		ancientCodes, ancientLub,
		loadN, speedRPM, tempCelsius, simHours,
	)
	modernResult := ec.mc.CompareMaterialsGeneric(
		bearingDiameterMM, bearingDiameterMM*1.5, widthMM,
		modernCodes, modernLub,
		loadN, speedRPM, tempCelsius, simHours,
	)

	result := &models.CrossEraComparisonResult{
		ReferenceLoad:   loadN,
		ReferenceSpeed:  speedRPM,
		ReferenceTemp:   tempCelsius,
		SimulationHours: simHours,
		BearingDiameter: bearingDiameterMM,
		GeneratedAt:     time.Now(),
		AllItems:        make([]models.EraComparisonItem, 0),
	}

	ancientBestLife := 0.0
	ancientWorstWear := 0.0
	if len(ancientResult) > 0 {
		result.AncientBest = &ancientResult[0]
		ancientBestLife = ancientResult[0].PredictedLifeHours
		for _, a := range ancientResult {
			if a.WearRateUmPerHour > ancientWorstWear {
				ancientWorstWear = a.WearRateUmPerHour
			}
			result.AllItems = append(result.AllItems, models.EraComparisonItem{
				MaterialCode:       a.MaterialCode,
				MaterialName:       a.MaterialName,
				Era:                a.Era,
				PredictedLifeHours: a.PredictedLifeHours,
				PredictedLifeYears: a.PredictedLifeYears,
				WearRateUmPerHour:  a.WearRateUmPerHour,
			})
		}
	}

	modernBestLife := 0.0
	modernBestWear := 1e9
	if len(modernResult) > 0 {
		result.ModernBest = &modernResult[0]
		modernBestLife = modernResult[0].PredictedLifeHours
		for idx := range modernResult {
			m := modernResult[idx]
			if m.WearRateUmPerHour < modernBestWear {
				modernBestWear = m.WearRateUmPerHour
			}
			result.AllItems = append(result.AllItems, models.EraComparisonItem{
				MaterialCode:       m.MaterialCode,
				MaterialName:       m.MaterialName,
				Era:                m.Era,
				PredictedLifeHours: m.PredictedLifeHours,
				PredictedLifeYears: m.PredictedLifeYears,
				WearRateUmPerHour:  m.WearRateUmPerHour,
			})
		}
	}

	sort.Slice(result.AllItems, func(i, j int) bool {
		return result.AllItems[i].PredictedLifeHours > result.AllItems[j].PredictedLifeHours
	})
	for idx := range result.AllItems {
		result.AllItems[idx].EraRank = idx + 1
	}

	if ancientBestLife > 0 {
		result.LifeImprovementX = modernBestLife / ancientBestLife
	}
	if ancientWorstWear > 0 {
		result.WearReductionPct = (ancientWorstWear - modernBestWear) / ancientWorstWear * 100.0
	}

	result.InsightSummary = ec.generateEraInsights(result, ancientBestLife, modernBestLife)

	return result
}

func (ec *EraComparator) generateEraInsights(
	result *models.CrossEraComparisonResult,
	ancientBestLife, modernBestLife float64,
) []string {
	insights := make([]string, 0)

	if result.LifeImprovementX >= 50 {
		insights = append(insights,
			"现代轴承技术将寿命提升了约"+formatNumber(result.LifeImprovementX, 0)+"倍，相当于从使用3个月提升到使用10年以上。")
	} else if result.LifeImprovementX >= 10 {
		insights = append(insights,
			"现代轴承技术实现了约"+formatNumber(result.LifeImprovementX, 0)+"倍的寿命提升。")
	}

	if result.WearReductionPct >= 95 {
		insights = append(insights,
			"现代材料与润滑组合可将磨损率降低约"+formatNumber(result.WearReductionPct, 0)+"%，几乎消除了粘着磨损。")
	}

	if result.AncientBest != nil {
		insights = append(insights,
			"古代最优方案："+result.AncientBest.MaterialName+
				"，预估寿命约"+formatNumber(result.AncientBest.PredictedLifeYears, 1)+"年。"+
				"印证了古代工匠对耐磨材料的经验选择。")
	}

	if result.ModernBest != nil {
		insights = append(insights,
			"现代最优方案："+result.ModernBest.MaterialName+
				"，预估寿命约"+formatNumber(result.ModernBest.PredictedLifeYears, 0)+"年。"+
				"体现了材料科学与精密制造的巨大进步。")
	}

	insights = append(insights,
		"古代工匠采用经验试错法选择材料和润滑剂，"+
			"现代工程则基于摩擦学理论精确计算，效率提升了数个数量级。")

	return insights
}

func formatNumber(x float64, decimals int) string {
	format := ""
	switch decimals {
	case 0:
		format = "%.0f"
	case 1:
		format = "%.1f"
	default:
		format = "%.2f"
	}
	return fmt.Sprintf(format, x)
}

func (ec *EraComparator) getDefaultAncientLubricant() config.Lubricant {
	if lub, ok := ec.lubricants.GetLubricant("vegetable_tung"); ok {
		return lub
	}
	return ec.getDefaultLubricant()
}

func (ec *EraComparator) getDefaultModernLubricant() config.Lubricant {
	if lub, ok := ec.lubricants.GetLubricant("mineral_synthetic_pao"); ok {
		return lub
	}
	return ec.getDefaultLubricant()
}

func (ec *EraComparator) getDefaultLubricant() config.Lubricant {
	if len(ec.lubricants.Lubricants) > 0 {
		return ec.lubricants.Lubricants[0]
	}
	return config.Lubricant{
		ViscosityPaSAt40C:      0.03,
		PressureViscosityCoeff: 2.2e-8,
		EHLBoostFactor:         1.0,
		WearReductionRatio:     0.5,
	}
}
