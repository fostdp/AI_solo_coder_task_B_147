package lubricant_analyzer

import (
	"sort"
	"time"

	"noria-bearing-system/internal/config"
	"noria-bearing-system/internal/models"
	"noria-bearing-system/internal/modules/wearcalc"
)

type LubricantAnalyzer struct {
	materials  *config.BearingMaterialsConfig
	lubricants *config.LubricantsConfig
	wearParams *config.WearParamsConfig
	calculator *wearcalc.WearCalculator
}

func NewLubricantAnalyzer() *LubricantAnalyzer {
	return &LubricantAnalyzer{
		materials:  &config.AppConfig.Materials,
		lubricants: &config.AppConfig.Lubricants,
		wearParams: &config.AppConfig.WearParams,
		calculator: wearcalc.NewWearCalculator(2),
	}
}

func (la *LubricantAnalyzer) Close() {
	la.calculator.Close()
}

func (la *LubricantAnalyzer) CompareLubricants(
	baseBearing *models.Bearing,
	materialCode string,
	lubricantCodes []string,
	loadN, speedRPM, tempCelsius, simHours float64,
) *models.LubricantComparisonResult {
	mat, ok := la.materials.GetMaterial(materialCode)
	if !ok {
		mat = la.getDefaultAncientMaterial()
	}

	result := &models.LubricantComparisonResult{
		BaseBearingID:   baseBearing.ID,
		BaseMaterial:    mat.Code,
		ReferenceLoad:   loadN,
		ReferenceSpeed:  speedRPM,
		ReferenceTemp:   tempCelsius,
		SimulationHours: simHours,
		GeneratedAt:     time.Now(),
		Items:           make([]models.LubricantComparisonItem, 0),
	}

	isRolling := mat.Category == "rolling"
	dryInput := wearcalc.SimInput{
		InnerDiameterMM: baseBearing.InnerDiameter,
		OuterDiameterMM: baseBearing.OuterDiameter,
		WidthMM:         baseBearing.Width,
		LoadN:           loadN,
		SpeedRPM:        speedRPM,
		TempCelsius:     tempCelsius,
		Hours:           simHours,
		HardnessHV:      mat.HardnessHVNominal,
		ArchardK:        mat.ArchardKBase,
		SurfaceRMS:      mat.SurfaceRoughnessRMS,
		ElasticModPa:    mat.ElasticModulusPa,
		ViscosityPaS:    0.001,
		Viscosity40C:    0,
		WaltherA:        0,
		WaltherB:        0,
		PressureCoeff:   la.wearParams.PressureViscosityCoefficient,
		EHLBoost:        0.1,
		WearReduction:   0.0,
		WearResistance:  mat.WearResistanceFactor,
		IsRolling:       isRolling,
	}
	dryOut := la.calculator.SimulateWear(dryInput)

	inputs := make([]wearcalc.SimInput, 0, len(lubricantCodes))
	validLubs := make([]config.Lubricant, 0, len(lubricantCodes))

	for _, code := range lubricantCodes {
		lub, ok := la.lubricants.GetLubricant(code)
		if !ok {
			continue
		}
		validLubs = append(validLubs, lub)
		inputs = append(inputs, wearcalc.SimInput{
			InnerDiameterMM: baseBearing.InnerDiameter,
			OuterDiameterMM: baseBearing.OuterDiameter,
			WidthMM:         baseBearing.Width,
			LoadN:           loadN,
			SpeedRPM:        speedRPM,
			TempCelsius:     tempCelsius,
			Hours:           simHours,
			HardnessHV:      mat.HardnessHVNominal,
			ArchardK:        mat.ArchardKBase,
			SurfaceRMS:      mat.SurfaceRoughnessRMS,
			ElasticModPa:    mat.ElasticModulusPa,
			ViscosityPaS:    lub.ViscosityPaSAt40C,
			Viscosity40C:    lub.Viscosity40C,
			WaltherA:        lub.WaltherIntercept,
			WaltherB:        lub.WaltherSlope,
			PressureCoeff:   lub.PressureViscosityCoeff,
			EHLBoost:        lub.EHLBoostFactor,
			WearReduction:   lub.WearReductionRatio,
			WearResistance:  mat.WearResistanceFactor,
			IsRolling:       isRolling,
		})
	}

	outputs := wearcalc.BatchSimulate(la.calculator, inputs)
	regimeCN := map[string]string{
		"full_film": "全膜弹流润滑",
		"mixed":     "混合润滑",
		"boundary":  "边界润滑",
	}

	for i, lub := range validLubs {
		simOut := outputs[i]

		wearReductionPct := 0.0
		lifeExtensionPct := 0.0
		if dryOut.TotalWearUm > 0 {
			wearReductionPct = (dryOut.TotalWearUm - simOut.TotalWearUm) / dryOut.TotalWearUm * 100.0
		}
		if simOut.LifeHours > 0 && dryOut.LifeHours > 0 {
			lifeExtensionPct = (simOut.LifeHours - dryOut.LifeHours) / dryOut.LifeHours * 100.0
		}

		recommendedFreq, hasFreq := la.lubricants.RecommendedLubricationFreq[mat.Code]
		if !hasFreq {
			recommendedFreq = lub.MaxLubricationLifeHours * 0.5
		}

		item := models.LubricantComparisonItem{
			LubricantCode:        lub.Code,
			LubricantName:        lub.NameCN,
			Category:             lub.Category,
			Era:                  lub.Era,
			ViscosityAt40C:       lub.Viscosity40C,
			ViscosityIndex:       lub.ViscosityIndex,
			TotalWearMicrom:      simOut.TotalWearUm,
			WearRateUmPerHour:    simOut.WearRateUmPerHour,
			PredictedLifeHours:   simOut.LifeHours,
			PredictedLifeYears:   simOut.LifeYears,
			EHLMeanLambda:        simOut.EHLMeanLambda,
			LubricationRegime:    regimeCN[simOut.Regime],
			WearReductionVsDry:   wearReductionPct,
			LifeExtensionVsDry:   lifeExtensionPct,
			RecommendedFreqHours: recommendedFreq,
			HistoricalNote:       lub.HistoricalNote,
		}
		result.Items = append(result.Items, item)
	}

	sort.Slice(result.Items, func(i, j int) bool {
		return result.Items[i].PredictedLifeHours > result.Items[j].PredictedLifeHours
	})

	bestLife := 0.0
	if len(result.Items) > 0 {
		bestLife = result.Items[0].PredictedLifeHours
	}
	for idx := range result.Items {
		result.Items[idx].Rank = idx + 1
		if bestLife > 0 {
			result.Items[idx].LifeRatioVsBest = result.Items[idx].PredictedLifeHours / bestLife
		}
	}

	return result
}

func (la *LubricantAnalyzer) GetAllLubricants() []config.Lubricant {
	return la.lubricants.Lubricants
}

func (la *LubricantAnalyzer) GetLubricantsByCategory(cat string) []config.Lubricant {
	return la.lubricants.ListByCategory(cat)
}

func (la *LubricantAnalyzer) getDefaultAncientMaterial() config.BearingMaterial {
	if mat, ok := la.materials.GetMaterial("bronze_ancient"); ok {
		return mat
	}
	return la.getDefaultMaterial()
}

func (la *LubricantAnalyzer) getDefaultMaterial() config.BearingMaterial {
	if len(la.materials.Materials) > 0 {
		return la.materials.Materials[0]
	}
	return config.BearingMaterial{
		HardnessHVNominal:    110,
		ArchardKBase:         1.5e-8,
		SurfaceRoughnessRMS:  0.8e-6,
		ElasticModulusPa:     1.0e11,
		WearResistanceFactor: 0.85,
	}
}
