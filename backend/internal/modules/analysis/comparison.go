package analysis

import (
	"fmt"
	"math"
	"sort"
	"time"

	"noria-bearing-system/internal/config"
	"noria-bearing-system/internal/models"
)

type ComparisonEngine struct {
	wearParams  *config.WearParamsConfig
	lubrication *config.LubricationConfig
	materials   *config.BearingMaterialsConfig
	lubricants  *config.LubricantsConfig
}

func NewComparisonEngine() *ComparisonEngine {
	return &ComparisonEngine{
		wearParams:  &config.AppConfig.WearParams,
		lubrication: &config.AppConfig.Lubrication,
		materials:   &config.AppConfig.Materials,
		lubricants:  &config.AppConfig.Lubricants,
	}
}

type simInput struct {
	innerDiameterMM float64
	outerDiameterMM float64
	widthMM         float64
	loadN           float64
	speedRPM        float64
	tempCelsius     float64
	hours           float64
	hardnessHV      float64
	archardK        float64
	surfaceRMS      float64
	elasticModPa    float64
	viscosityPaS    float64
	viscosity40C    float64
	waltherA        float64
	waltherB        float64
	pressureCoeff   float64
	ehlBoost        float64
	wearReduction   float64
	wearResistance  float64
	isRolling       bool
}

type simOutput struct {
	totalWearUm        float64
	wearRateUmPerHour  float64
	ehlMeanLambda      float64
	contactPressureMPa float64
	archardVolumeM3    float64
	lifeHours          float64
	lifeYears          float64
	wearLimitUm        float64
	regime             string
}

const wearLimitDefaultUm = 500.0

func (ce *ComparisonEngine) calculateViscosityWalther(tempCelsius, waltherA, waltherB, viscosity40C, viscosityPaS40C float64) float64 {
	if tempCelsius <= -273.15 {
		return viscosityPaS40C
	}

	kelvin := tempCelsius + 273.15
	logKelvin := math.Log10(kelvin)
	logLogVal := waltherA - waltherB*logKelvin

	if logLogVal <= 0 {
		return viscosityPaS40C
	}

	logVal := math.Pow(10, logLogVal)
	kinematicViscosity := math.Pow(10, logVal) - 0.8

	if kinematicViscosity <= 0 {
		return viscosityPaS40C
	}

	ratio := kinematicViscosity / viscosity40C
	if ratio <= 0 {
		return viscosityPaS40C
	}

	return viscosityPaS40C * ratio
}

func (ce *ComparisonEngine) simulateWear(in simInput) simOutput {
	out := simOutput{}

	innerRadius := in.innerDiameterMM / 2.0 / 1000.0
	outerRadius := in.outerDiameterMM / 2.0 / 1000.0
	effectiveRadius := (innerRadius + outerRadius) / 2.0
	widthMeters := in.widthMM / 1000.0

	contactArea := math.Pi * (outerRadius*outerRadius - innerRadius*innerRadius)
	if contactArea <= 0 {
		contactArea = effectiveRadius * 2 * math.Pi * widthMeters
	}

	contactPressurePa := in.loadN / contactArea
	out.contactPressureMPa = contactPressurePa / 1e6

	rpmToRadPerSec := 2.0 * math.Pi / 60.0
	angularVelocity := in.speedRPM * rpmToRadPerSec
	slidingVelocity := effectiveRadius * angularVelocity

	if in.isRolling {
		slidingVelocity *= 0.02
	}

	periodSeconds := in.hours * 3600.0
	slidingDistance := slidingVelocity * periodSeconds

	hardnessPa := in.hardnessHV * ce.wearParams.HardnessConversionFactor

	viscosity := in.viscosityPaS
	if viscosity <= 0 {
		viscosity = 0.03
	}

	if in.waltherA > 0 && in.waltherB > 0 && in.viscosity40C > 0 {
		viscosity = ce.calculateViscosityWalther(
			in.tempCelsius,
			in.waltherA,
			in.waltherB,
			in.viscosity40C,
			in.viscosityPaS,
		)
	} else {
		refTemp := ce.wearParams.EHLReferenceTempCelsius
		tempCorrection := math.Exp(-0.05 * (in.tempCelsius - refTemp) * 0.693 / 10.0)
		viscosity = in.viscosityPaS * tempCorrection
	}
	effectiveViscosity := viscosity

	filmThickness := ce.calculateEHLFilm(
		effectiveRadius,
		in.speedRPM,
		effectiveViscosity,
		in.loadN,
		in.elasticModPa,
		in.pressureCoeff,
	)

	filmThickness *= in.ehlBoost

	out.ehlMeanLambda = filmThickness / in.surfaceRMS
	if out.ehlMeanLambda < ce.lubrication.EHLCorrection.LambdaMinClamp {
		out.ehlMeanLambda = ce.lubrication.EHLCorrection.LambdaMinClamp
	}
	if out.ehlMeanLambda > ce.lubrication.EHLCorrection.LambdaMaxClamp {
		out.ehlMeanLambda = ce.lubrication.EHLCorrection.LambdaMaxClamp
	}

	switch {
	case out.ehlMeanLambda >= ce.wearParams.FullFilmLambdaThreshold:
		out.regime = "full_film"
	case out.ehlMeanLambda >= ce.wearParams.MixedLubricationLambdaThreshold:
		out.regime = "mixed"
	default:
		out.regime = "boundary"
	}

	wearCoefficient := ce.wearCoefficientForLambda(out.ehlMeanLambda)
	wearCoefficient *= (1.0 - in.wearReduction*0.8)
	wearCoefficient = wearCoefficient / in.wearResistance

	tempFactor := 1.0
	if in.tempCelsius > refTemp {
		tempFactor = 1.0 + ce.wearParams.TempCorrectionFactorPerDegree*(in.tempCelsius-refTemp)
	}

	archardVolume := in.archardK * wearCoefficient * in.loadN * slidingDistance / hardnessPa * tempFactor
	if in.isRolling {
		archardVolume *= 0.05
	}

	wearDepthMeters := archardVolume / contactArea
	out.totalWearUm = wearDepthMeters * 1e6
	out.archardVolumeM3 = archardVolume

	if in.hours > 0 {
		out.wearRateUmPerHour = out.totalWearUm / in.hours
	}

	wearLimit := wearLimitDefaultUm
	if in.innerDiameterMM >= 200 {
		wearLimit = 1000.0
	} else if in.innerDiameterMM >= 100 {
		wearLimit = 750.0
	}

	if in.isRolling {
		wearLimit *= 0.3
	}

	out.wearLimitUm = wearLimit

	if out.wearRateUmPerHour > 0 {
		out.lifeHours = wearLimit / out.wearRateUmPerHour * 0.9
	} else {
		out.lifeHours = in.hours * 1000
	}

	if out.lifeHours > 200000 {
		out.lifeHours = 200000
	}

	out.lifeYears = out.lifeHours / 8760.0

	return out
}

func (ce *ComparisonEngine) calculateEHLFilm(
	effRadius, speedRPM, viscosity, load, elasticMod, pressureCoeff float64,
) float64 {
	if effRadius <= 0 || viscosity <= 0 {
		return 1e-7
	}

	angularVel := speedRPM * 2.0 * math.Pi / 60.0
	entrainmentSpeed := effRadius * angularVel

	if entrainmentSpeed <= 0.001 {
		entrainmentSpeed = 0.001
	}

	G := elasticMod * pressureCoeff
	U := viscosity * entrainmentSpeed / (elasticMod * effRadius)
	W := load / (elasticMod * effRadius * effRadius)

	if W <= 0 {
		W = 1e-10
	}
	if U <= 1e-18 {
		U = 1e-18
	}

	dowsonHigginson := ce.lubrication.EHLCorrection.DowsonHigginsonCoefficient *
		effRadius *
		math.Pow(U, 0.68) *
		math.Pow(G, 0.49) *
		math.Pow(W, -0.073) *
		(1.0 - 0.61*math.Exp(-0.73*ce.lubrication.EHLCorrection.AlternativeFormulaCoefficient*U*1e12))

	film := dowsonHigginson
	if film < 0.05e-6 {
		film = 0.05e-6
	}
	if film > 20e-6 {
		film = 20e-6
	}

	return film
}

func (ce *ComparisonEngine) wearCoefficientForLambda(lambda float64) float64 {
	factors := ce.wearParams.WearCoefficientFactors
	fullFilmThreshold := ce.wearParams.FullFilmLambdaThreshold
	mixedThreshold := ce.wearParams.MixedLubricationLambdaThreshold

	if lambda >= fullFilmThreshold {
		return factors["full_film"]
	} else if lambda >= mixedThreshold {
		ratio := (lambda - mixedThreshold) / (fullFilmThreshold - mixedThreshold)
		return factors["mixed_max"]*(1.0-ratio) + factors["mixed_min"]*ratio
	} else {
		ratio := lambda / mixedThreshold
		if ratio < 0 {
			ratio = 0
		}
		return factors["boundary_max"]*(1.0-ratio) + factors["boundary_min"]*ratio
	}
}

func (ce *ComparisonEngine) CompareMaterials(
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

	baseLubricant := ce.getDefaultAncientLubricant(baseBearing)

	for _, code := range materialCodes {
		mat, ok := ce.materials.GetMaterial(code)
		if !ok {
			continue
		}

		isRolling := mat.Category == "rolling"

		simIn := simInput{
			innerDiameterMM: baseBearing.InnerDiameter,
			outerDiameterMM: baseBearing.OuterDiameter,
			widthMM:         baseBearing.Width,
			loadN:           loadN,
			speedRPM:        speedRPM,
			tempCelsius:     tempCelsius,
			hours:           simHours,
			hardnessHV:      mat.HardnessHVNominal,
			archardK:        mat.ArchardKBase,
			surfaceRMS:      mat.SurfaceRoughnessRMS,
			elasticModPa:    mat.ElasticModulusPa,
			viscosityPaS:    baseLubricant.ViscosityPaSAt40C,
			viscosity40C:    baseLubricant.Viscosity40C,
			waltherA:        baseLubricant.WaltherIntercept,
			waltherB:        baseLubricant.WaltherSlope,
			pressureCoeff:   baseLubricant.PressureViscosityCoeff,
			ehlBoost:        baseLubricant.EHLBoostFactor,
			wearReduction:   baseLubricant.WearReductionRatio,
			wearResistance:  mat.WearResistanceFactor,
			isRolling:       isRolling,
		}

		simOut := ce.simulateWear(simIn)

		item := models.MaterialComparisonItem{
			MaterialCode:          mat.Code,
			MaterialName:          mat.NameCN,
			Era:                   mat.Era,
			Category:              mat.Category,
			HardnessHV:            mat.HardnessHVNominal,
			TotalWearMicrom:       simOut.totalWearUm,
			WearRateUmPerHour:     simOut.wearRateUmPerHour,
			PredictedLifeHours:    simOut.lifeHours,
			PredictedLifeYears:    simOut.lifeYears,
			EHLMeanLambda:         simOut.ehlMeanLambda,
			MaxContactPressureMPa: simOut.contactPressureMPa,
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

func (ce *ComparisonEngine) CompareLubricants(
	baseBearing *models.Bearing,
	materialCode string,
	lubricantCodes []string,
	loadN, speedRPM, tempCelsius, simHours float64,
) *models.LubricantComparisonResult {
	mat, ok := ce.materials.GetMaterial(materialCode)
	if !ok {
		mat = ce.getDefaultAncientMaterial()
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
	dryInput := simInput{
		innerDiameterMM: baseBearing.InnerDiameter,
		outerDiameterMM: baseBearing.OuterDiameter,
		widthMM:         baseBearing.Width,
		loadN:           loadN,
		speedRPM:        speedRPM,
		tempCelsius:     tempCelsius,
		hours:           simHours,
		hardnessHV:      mat.HardnessHVNominal,
		archardK:        mat.ArchardKBase,
		surfaceRMS:      mat.SurfaceRoughnessRMS,
		elasticModPa:    mat.ElasticModulusPa,
		viscosityPaS:    0.001,
		viscosity40C:    0,
		waltherA:        0,
		waltherB:        0,
		pressureCoeff:   ce.wearParams.PressureViscosityCoefficient,
		ehlBoost:        0.1,
		wearReduction:   0.0,
		wearResistance:  mat.WearResistanceFactor,
		isRolling:       isRolling,
	}
	dryOut := ce.simulateWear(dryInput)

	for _, code := range lubricantCodes {
		lub, ok := ce.lubricants.GetLubricant(code)
		if !ok {
			continue
		}

		simIn := simInput{
			innerDiameterMM: baseBearing.InnerDiameter,
			outerDiameterMM: baseBearing.OuterDiameter,
			widthMM:         baseBearing.Width,
			loadN:           loadN,
			speedRPM:        speedRPM,
			tempCelsius:     tempCelsius,
			hours:           simHours,
			hardnessHV:      mat.HardnessHVNominal,
			archardK:        mat.ArchardKBase,
			surfaceRMS:      mat.SurfaceRoughnessRMS,
			elasticModPa:    mat.ElasticModulusPa,
			viscosityPaS:    lub.ViscosityPaSAt40C,
			viscosity40C:    lub.Viscosity40C,
			waltherA:        lub.WaltherIntercept,
			waltherB:        lub.WaltherSlope,
			pressureCoeff:   lub.PressureViscosityCoeff,
			ehlBoost:        lub.EHLBoostFactor,
			wearReduction:   lub.WearReductionRatio,
			wearResistance:  mat.WearResistanceFactor,
			isRolling:       isRolling,
		}

		simOut := ce.simulateWear(simIn)

		wearReductionPct := 0.0
		lifeExtensionPct := 0.0
		if dryOut.totalWearUm > 0 {
			wearReductionPct = (dryOut.totalWearUm - simOut.totalWearUm) / dryOut.totalWearUm * 100.0
		}
		if simOut.lifeHours > 0 && dryOut.lifeHours > 0 {
			lifeExtensionPct = (simOut.lifeHours - dryOut.lifeHours) / dryOut.lifeHours * 100.0
		}

		regimeCN := map[string]string{
			"full_film": "全膜弹流润滑",
			"mixed":     "混合润滑",
			"boundary":  "边界润滑",
		}

		recommendedFreq, hasFreq := ce.lubricants.RecommendedLubricationFreq[mat.Code]
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
			TotalWearMicrom:      simOut.totalWearUm,
			WearRateUmPerHour:    simOut.wearRateUmPerHour,
			PredictedLifeHours:   simOut.lifeHours,
			PredictedLifeYears:   simOut.lifeYears,
			EHLMeanLambda:        simOut.ehlMeanLambda,
			LubricationRegime:    regimeCN[simOut.regime],
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

func (ce *ComparisonEngine) CrossEraComparison(
	bearingDiameterMM, widthMM, loadN, speedRPM, tempCelsius, simHours float64,
) *models.CrossEraComparisonResult {
	ancientCodes := []string{"wood_oak", "wood_ironbark", "bronze_ancient", "cast_iron_ancient", "wood_wrapped_copper"}
	modernCodes := []string{"modern_bushing_babbit", "modern_ball_bearing", "modern_roller_bearing"}

	ancientResult := ce.CompareMaterialsGeneric(
		bearingDiameterMM, bearingDiameterMM*1.5, widthMM,
		ancientCodes, ce.getDefaultAncientLubricant(nil),
		loadN, speedRPM, tempCelsius, simHours,
	)
	modernResult := ce.CompareMaterialsGeneric(
		bearingDiameterMM, bearingDiameterMM*1.5, widthMM,
		modernCodes, ce.getDefaultModernLubricant(),
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

	result.InsightSummary = ce.generateEraInsights(result, ancientBestLife, modernBestLife)

	return result
}

func (ce *ComparisonEngine) CompareMaterialsGeneric(
	innerDiameterMM, outerDiameterMM, widthMM float64,
	materialCodes []string,
	lubricant config.Lubricant,
	loadN, speedRPM, tempCelsius, simHours float64,
) []models.MaterialComparisonItem {
	items := make([]models.MaterialComparisonItem, 0)

	for _, code := range materialCodes {
		mat, ok := ce.materials.GetMaterial(code)
		if !ok {
			continue
		}

		isRolling := mat.Category == "rolling"

		simIn := simInput{
			innerDiameterMM: innerDiameterMM,
			outerDiameterMM: outerDiameterMM,
			widthMM:         widthMM,
			loadN:           loadN,
			speedRPM:        speedRPM,
			tempCelsius:     tempCelsius,
			hours:           simHours,
			hardnessHV:      mat.HardnessHVNominal,
			archardK:        mat.ArchardKBase,
			surfaceRMS:      mat.SurfaceRoughnessRMS,
			elasticModPa:    mat.ElasticModulusPa,
			viscosityPaS:    lubricant.ViscosityPaSAt40C,
			viscosity40C:    lubricant.Viscosity40C,
			waltherA:        lubricant.WaltherIntercept,
			waltherB:        lubricant.WaltherSlope,
			pressureCoeff:   lubricant.PressureViscosityCoeff,
			ehlBoost:        lubricant.EHLBoostFactor,
			wearReduction:   lubricant.WearReductionRatio,
			wearResistance:  mat.WearResistanceFactor,
			isRolling:       isRolling,
		}

		simOut := ce.simulateWear(simIn)

		item := models.MaterialComparisonItem{
			MaterialCode:          mat.Code,
			MaterialName:          mat.NameCN,
			Era:                   mat.Era,
			Category:              mat.Category,
			HardnessHV:            mat.HardnessHVNominal,
			TotalWearMicrom:       simOut.totalWearUm,
			WearRateUmPerHour:     simOut.wearRateUmPerHour,
			PredictedLifeHours:    simOut.lifeHours,
			PredictedLifeYears:    simOut.lifeYears,
			EHLMeanLambda:         simOut.ehlMeanLambda,
			MaxContactPressureMPa: simOut.contactPressureMPa,
			HistoricalNote:        mat.HistoricalNote,
			TypicalApplications:   mat.TypicalApplications,
		}
		items = append(items, item)
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].PredictedLifeHours > items[j].PredictedLifeHours
	})

	bestLife := 0.0
	if len(items) > 0 {
		bestLife = items[0].PredictedLifeHours
	}
	for idx := range items {
		items[idx].Rank = idx + 1
		if bestLife > 0 {
			items[idx].LifeRatioVsBest = items[idx].PredictedLifeHours / bestLife
		}
	}

	return items
}

func (ce *ComparisonEngine) generateEraInsights(
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

func (ce *ComparisonEngine) getDefaultAncientLubricant(_ *models.Bearing) config.Lubricant {
	if lub, ok := ce.lubricants.GetLubricant("vegetable_tung"); ok {
		return lub
	}
	return ce.getDefaultLubricant()
}

func (ce *ComparisonEngine) getDefaultModernLubricant() config.Lubricant {
	if lub, ok := ce.lubricants.GetLubricant("mineral_synthetic_pao"); ok {
		return lub
	}
	return ce.getDefaultLubricant()
}

func (ce *ComparisonEngine) getDefaultLubricant() config.Lubricant {
	if len(ce.lubricants.Lubricants) > 0 {
		return ce.lubricants.Lubricants[0]
	}
	return config.Lubricant{
		ViscosityPaSAt40C:      0.03,
		PressureViscosityCoeff: 2.2e-8,
		EHLBoostFactor:         1.0,
		WearReductionRatio:     0.5,
	}
}

func (ce *ComparisonEngine) getDefaultAncientMaterial() config.BearingMaterial {
	if mat, ok := ce.materials.GetMaterial("bronze_ancient"); ok {
		return mat
	}
	return ce.getDefaultMaterial()
}

func (ce *ComparisonEngine) getDefaultMaterial() config.BearingMaterial {
	if len(ce.materials.Materials) > 0 {
		return ce.materials.Materials[0]
	}
	return config.BearingMaterial{
		HardnessHVNominal:    110,
		ArchardKBase:         1.5e-8,
		SurfaceRoughnessRMS:  0.8e-6,
		ElasticModulusPa:     1.0e11,
		WearResistanceFactor: 0.85,
	}
}

func (ce *ComparisonEngine) GetAllMaterials() []config.BearingMaterial {
	return ce.materials.Materials
}

func (ce *ComparisonEngine) GetAllLubricants() []config.Lubricant {
	return ce.lubricants.Lubricants
}

func (ce *ComparisonEngine) GetMaterialsByEra(era string) []config.BearingMaterial {
	return ce.materials.ListByEra(era)
}

func (ce *ComparisonEngine) GetLubricantsByCategory(cat string) []config.Lubricant {
	return ce.lubricants.ListByCategory(cat)
}

