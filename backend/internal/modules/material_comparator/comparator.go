package material_comparator

import (
	"sort"
	"time"

	"noria-bearing-system/internal/config"
	"noria-bearing-system/internal/models"
	"noria-bearing-system/internal/modules/wearcalc"
)

type MaterialComparator struct {
	materials  *config.BearingMaterialsConfig
	lubricants *config.LubricantsConfig
	calculator *wearcalc.WearCalculator
}

func NewMaterialComparator() *MaterialComparator {
	return &MaterialComparator{
		materials:  &config.AppConfig.Materials,
		lubricants: &config.AppConfig.Lubricants,
		calculator: wearcalc.NewWearCalculator(2),
	}
}

func (mc *MaterialComparator) Close() {
	mc.calculator.Close()
}

func (mc *MaterialComparator) CompareMaterials(
	baseBearing *models.Bearing,
	materialCodes []string,
	loadN, speedRPM, tempCelsius, simHours float64,
) *models.MaterialComparisonResult {
	result := &models.MaterialComparisonResult{
		BaseBearingID:   baseBearing.ID,
		ReferenceLoad:   loadN,
		ReferenceSpeed:  speedRPM,
		ReferenceTemp:   tempCelsius,
		SimulationHours: simHours,
		GeneratedAt:     time.Now(),
		Items:           make([]models.MaterialComparisonItem, 0),
	}

	baseLubricant := mc.getDefaultAncientLubricant()

	inputs := make([]wearcalc.SimInput, 0, len(materialCodes))
	validMats := make([]config.BearingMaterial, 0, len(materialCodes))

	for _, code := range materialCodes {
		mat, ok := mc.materials.GetMaterial(code)
		if !ok {
			continue
		}
		validMats = append(validMats, mat)

		isRolling := mat.Category == "rolling"
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
			ViscosityPaS:    baseLubricant.ViscosityPaSAt40C,
			Viscosity40C:    baseLubricant.Viscosity40C,
			WaltherA:        baseLubricant.WaltherIntercept,
			WaltherB:        baseLubricant.WaltherSlope,
			PressureCoeff:   baseLubricant.PressureViscosityCoeff,
			EHLBoost:        baseLubricant.EHLBoostFactor,
			WearReduction:   baseLubricant.WearReductionRatio,
			WearResistance:  mat.WearResistanceFactor,
			IsRolling:       isRolling,
		})
	}

	outputs := wearcalc.BatchSimulate(mc.calculator, inputs)

	for i, mat := range validMats {
		simOut := outputs[i]
		item := models.MaterialComparisonItem{
			MaterialCode:          mat.Code,
			MaterialName:          mat.NameCN,
			Era:                   mat.Era,
			Category:              mat.Category,
			HardnessHV:            mat.HardnessHVNominal,
			TotalWearMicrom:       simOut.TotalWearUm,
			WearRateUmPerHour:     simOut.WearRateUmPerHour,
			PredictedLifeHours:    simOut.LifeHours,
			PredictedLifeYears:    simOut.LifeYears,
			EHLMeanLambda:         simOut.EHLMeanLambda,
			MaxContactPressureMPa: simOut.ContactPressureMPa,
			HistoricalNote:        mat.HistoricalNote,
			TypicalApplications:   mat.TypicalApplications,
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

func (mc *MaterialComparator) CompareMaterialsGeneric(
	innerDiameterMM, outerDiameterMM, widthMM float64,
	materialCodes []string,
	lubricant config.Lubricant,
	loadN, speedRPM, tempCelsius, simHours float64,
) []models.MaterialComparisonItem {
	items := make([]models.MaterialComparisonItem, 0)

	inputs := make([]wearcalc.SimInput, 0)
	validMats := make([]config.BearingMaterial, 0)

	for _, code := range materialCodes {
		mat, ok := mc.materials.GetMaterial(code)
		if !ok {
			continue
		}
		validMats = append(validMats, mat)
		isRolling := mat.Category == "rolling"

		inputs = append(inputs, wearcalc.SimInput{
			InnerDiameterMM: innerDiameterMM,
			OuterDiameterMM: outerDiameterMM,
			WidthMM:         widthMM,
			LoadN:           loadN,
			SpeedRPM:        speedRPM,
			TempCelsius:     tempCelsius,
			Hours:           simHours,
			HardnessHV:      mat.HardnessHVNominal,
			ArchardK:        mat.ArchardKBase,
			SurfaceRMS:      mat.SurfaceRoughnessRMS,
			ElasticModPa:    mat.ElasticModulusPa,
			ViscosityPaS:    lubricant.ViscosityPaSAt40C,
			Viscosity40C:    lubricant.Viscosity40C,
			WaltherA:        lubricant.WaltherIntercept,
			WaltherB:        lubricant.WaltherSlope,
			PressureCoeff:   lubricant.PressureViscosityCoeff,
			EHLBoost:        lubricant.EHLBoostFactor,
			WearReduction:   lubricant.WearReductionRatio,
			WearResistance:  mat.WearResistanceFactor,
			IsRolling:       isRolling,
		})
	}

	outputs := wearcalc.BatchSimulate(mc.calculator, inputs)

	for i, mat := range validMats {
		simOut := outputs[i]
		items = append(items, models.MaterialComparisonItem{
			MaterialCode:          mat.Code,
			MaterialName:          mat.NameCN,
			Era:                   mat.Era,
			Category:              mat.Category,
			HardnessHV:            mat.HardnessHVNominal,
			TotalWearMicrom:       simOut.TotalWearUm,
			WearRateUmPerHour:     simOut.WearRateUmPerHour,
			PredictedLifeHours:    simOut.LifeHours,
			PredictedLifeYears:    simOut.LifeYears,
			EHLMeanLambda:         simOut.EHLMeanLambda,
			MaxContactPressureMPa: simOut.ContactPressureMPa,
			HistoricalNote:        mat.HistoricalNote,
			TypicalApplications:   mat.TypicalApplications,
		})
	}

	return items
}

func (mc *MaterialComparator) GetAllMaterials() []config.BearingMaterial {
	return mc.materials.Materials
}

func (mc *MaterialComparator) GetMaterialsByEra(era string) []config.BearingMaterial {
	return mc.materials.ListByEra(era)
}

func (mc *MaterialComparator) getDefaultAncientLubricant() config.Lubricant {
	if lub, ok := mc.lubricants.GetLubricant("vegetable_tung"); ok {
		return lub
	}
	return mc.getDefaultLubricant()
}

func (mc *MaterialComparator) getDefaultLubricant() config.Lubricant {
	if len(mc.lubricants.Lubricants) > 0 {
		return mc.lubricants.Lubricants[0]
	}
	return config.Lubricant{
		ViscosityPaSAt40C:      0.03,
		PressureViscosityCoeff: 2.2e-8,
		EHLBoostFactor:         1.0,
		WearReductionRatio:     0.5,
	}
}

func (mc *MaterialComparator) GetDefaultAncientMaterial() config.BearingMaterial {
	if mat, ok := mc.materials.GetMaterial("bronze_ancient"); ok {
		return mat
	}
	return mc.getDefaultMaterial()
}

func (mc *MaterialComparator) getDefaultMaterial() config.BearingMaterial {
	if len(mc.materials.Materials) > 0 {
		return mc.materials.Materials[0]
	}
	return config.BearingMaterial{
		HardnessHVNominal:    110,
		ArchardKBase:         1.5e-8,
		SurfaceRoughnessRMS:  0.8e-6,
		ElasticModulusPa:     1.0e11,
		WearResistanceFactor: 0.85,
	}
}
