package vr_maintenance

import (
	"context"
	"errors"
	"fmt"
	"time"

	"noria-bearing-system/internal/config"
	"noria-bearing-system/internal/database"
	"noria-bearing-system/internal/models"
	"noria-bearing-system/internal/modules/lubricant_analyzer"
	"noria-bearing-system/internal/modules/material_comparator"
)

type VRMaintenanceManager struct {
	db             *database.Database
	mc             *material_comparator.MaterialComparator
	la             *lubricant_analyzer.LubricantAnalyzer
	materialsCfg   *config.BearingMaterialsConfig
	lubricantsCfg  *config.LubricantsConfig
	alertThreshold float64
}

func NewVRMaintenanceManager(db *database.Database) *VRMaintenanceManager {
	return &VRMaintenanceManager{
		db:             db,
		mc:             material_comparator.NewMaterialComparator(),
		la:             lubricant_analyzer.NewLubricantAnalyzer(),
		materialsCfg:   &config.AppConfig.Materials,
		lubricantsCfg:  &config.AppConfig.Lubricants,
		alertThreshold: 0.7,
	}
}

func (vrm *VRMaintenanceManager) Close() {
	vrm.mc.Close()
	vrm.la.Close()
}

type ReplaceBearingParams struct {
	BearingID       int
	NewMaterialCode string
	OperatorName    *string
	SessionID       *string
	Notes           *string
}

type AddLubricantParams struct {
	BearingID       int
	LubricantCode   string
	AmountML        float64
	OperatorName    *string
	SessionID       *string
	Notes           *string
}

func (vrm *VRMaintenanceManager) PreviewBearingReplacement(ctx context.Context, params ReplaceBearingParams) (*models.MaintenanceEffectPreview, error) {
	bearing, err := vrm.db.GetBearingByID(ctx, params.BearingID)
	if err != nil {
		return nil, fmt.Errorf("获取轴承信息失败: %w", err)
	}

	newMat, ok := vrm.materialsCfg.GetMaterial(params.NewMaterialCode)
	if !ok {
		return nil, errors.New("无效的新材料代码")
	}

	currentStatus, err := vrm.db.GetBearingLatestStatusByID(ctx, params.BearingID)
	if err != nil {
		return nil, fmt.Errorf("获取轴承状态失败: %w", err)
	}

	currentWearUm := 0.0
	if currentStatus.TotalWearMicrom != nil {
		currentWearUm = *currentStatus.TotalWearMicrom
	}
	currentLife := 0.0
	if currentStatus.PredictedRULHours != nil {
		currentLife = *currentStatus.PredictedRULHours
	}
	oldWearRate := 0.0
	if currentStatus.WearRateMicromPerHour != nil {
		oldWearRate = *currentStatus.WearRateMicromPerHour
	}

	loadN := 5000.0
	if currentStatus.RadialLoad != nil {
		loadN = *currentStatus.RadialLoad
	}
	speedRPM := 15.0
	if currentStatus.RotationalSpeed != nil {
		speedRPM = *currentStatus.RotationalSpeed
	}
	tempC := 40.0
	if currentStatus.Temperature != nil {
		tempC = *currentStatus.Temperature
	}

	materialCodes := []string{params.NewMaterialCode}
	compareResult := vrm.mc.CompareMaterials(bearing, materialCodes, loadN, speedRPM, tempC, 8760.0)

	newWearRate := 0.0
	projectedLife := 0.0
	if len(compareResult.Items) > 0 {
		newWearRate = compareResult.Items[0].WearRateUmPerHour
		projectedLife = compareResult.Items[0].PredictedLifeHours
	}

	remainingWearBudget := bearing.WearLimitMicrom - currentWearUm*0.1
	if remainingWearBudget < 0 {
		remainingWearBudget = bearing.WearLimitMicrom * 0.2
	}

	if newWearRate > 0 {
		projectedLife = remainingWearBudget / newWearRate * 0.85
	}

	lifeExtension := projectedLife - currentLife
	lifeExtensionPct := 0.0
	if currentLife > 0 {
		lifeExtensionPct = lifeExtension / currentLife * 100.0
	}

	costHint := vrm.generateReplacementCostHint(newMat.Code)

	return &models.MaintenanceEffectPreview{
		BearingID:          params.BearingID,
		CurrentWearUm:      currentWearUm,
		CurrentLifeHours:   currentLife,
		ProjectedLifeHours: projectedLife,
		LifeExtensionHours: lifeExtension,
		LifeExtensionPct:   lifeExtensionPct,
		NewWearRateUmHour:  newWearRate,
		OldWearRateUmHour:  oldWearRate,
		ActionSummary:      fmt.Sprintf("更换为 %s，预计磨损率降低 %.1f%%", newMat.NameCN, vrm.reductionPct(oldWearRate, newWearRate)),
		MaintenanceCostHint: &costHint,
	}, nil
}

func (vrm *VRMaintenanceManager) ExecuteBearingReplacement(ctx context.Context, params ReplaceBearingParams) (*models.MaintenanceRecord, error) {
	preview, err := vrm.PreviewBearingReplacement(ctx, params)
	if err != nil {
		return nil, err
	}

	newMat, _ := vrm.materialsCfg.GetMaterial(params.NewMaterialCode)

	bearing, err := vrm.db.GetBearingByID(ctx, params.BearingID)
	if err != nil {
		return nil, err
	}

	oldMaterial := vrm.guessMaterialFromBearing(bearing)

	rec := &models.MaintenanceRecord{
		BearingID:       params.BearingID,
		PerformedAt:     time.Now(),
		MaintenanceType: "replace_bearing",
		Action:          fmt.Sprintf("轴承更换: %s → %s", oldMaterial, newMat.NameCN),
		OldMaterialCode: &oldMaterial,
		NewMaterialCode: &params.NewMaterialCode,
		WearBeforeUm:    &preview.CurrentWearUm,
		OperatorName:    params.OperatorName,
		Notes:           params.Notes,
		UserSessionID:   params.SessionID,
	}

	err = vrm.saveMaintenanceRecord(ctx, rec)
	if err != nil {
		return nil, fmt.Errorf("保存维护记录失败: %w", err)
	}

	wearAfter := preview.CurrentWearUm * 0.05
	rec.WearAfterUm = &wearAfter
	_ = vrm.db.UpdateBearingMaterialAndHardness(ctx, params.BearingID, newMat.NameCN, newMat.HardnessHVNominal)

	return rec, nil
}

func (vrm *VRMaintenanceManager) PreviewLubricantAddition(ctx context.Context, params AddLubricantParams) (*models.MaintenanceEffectPreview, error) {
	bearing, err := vrm.db.GetBearingByID(ctx, params.BearingID)
	if err != nil {
		return nil, fmt.Errorf("获取轴承信息失败: %w", err)
	}

	lub, ok := vrm.lubricantsCfg.GetLubricant(params.LubricantCode)
	if !ok {
		return nil, errors.New("无效的润滑剂代码")
	}

	currentStatus, err := vrm.db.GetBearingLatestStatusByID(ctx, params.BearingID)
	if err != nil {
		return nil, fmt.Errorf("获取轴承状态失败: %w", err)
	}

	currentWearUm := 0.0
	if currentStatus.TotalWearMicrom != nil {
		currentWearUm = *currentStatus.TotalWearMicrom
	}
	currentLife := 0.0
	if currentStatus.PredictedRULHours != nil {
		currentLife = *currentStatus.PredictedRULHours
	}
	oldWearRate := 0.0
	if currentStatus.WearRateMicromPerHour != nil {
		oldWearRate = *currentStatus.WearRateMicromPerHour
	}

	loadN := 5000.0
	if currentStatus.RadialLoad != nil {
		loadN = *currentStatus.RadialLoad
	}
	speedRPM := 15.0
	if currentStatus.RotationalSpeed != nil {
		speedRPM = *currentStatus.RotationalSpeed
	}
	tempC := 40.0
	if currentStatus.Temperature != nil {
		tempC = *currentStatus.Temperature
	}

	materialCode := vrm.guessMaterialCode(bearing)
	lubCodes := []string{params.LubricantCode}
	compareResult := vrm.la.CompareLubricants(bearing, materialCode, lubCodes, loadN, speedRPM, tempC, 8760.0)

	newWearRate := 0.0
	projectedLife := 0.0
	if len(compareResult.Items) > 0 {
		newWearRate = compareResult.Items[0].WearRateUmPerHour
		projectedLife = compareResult.Items[0].PredictedLifeHours
	}

	remainingWearBudget := bearing.WearLimitMicrom - currentWearUm
	if remainingWearBudget < bearing.WearLimitMicrom*0.1 {
		remainingWearBudget = bearing.WearLimitMicrom * 0.3
	}

	adjustmentFactor := 0.6
	if oldWearRate > 0 && newWearRate < oldWearRate {
		adjustmentFactor = 0.75
	}
	if newWearRate > 0 {
		projectedLife = remainingWearBudget / newWearRate * adjustmentFactor
	}

	lifeExtension := projectedLife - currentLife
	if lifeExtension < 0 {
		lifeExtension = 0
		projectedLife = currentLife
	}
	lifeExtensionPct := 0.0
	if currentLife > 0 {
		lifeExtensionPct = lifeExtension / currentLife * 100.0
	}

	costHint := vrm.generateLubricantCostHint(lub.Code, params.AmountML)

	return &models.MaintenanceEffectPreview{
		BearingID:           params.BearingID,
		CurrentWearUm:       currentWearUm,
		CurrentLifeHours:    currentLife,
		ProjectedLifeHours:  projectedLife,
		LifeExtensionHours:  lifeExtension,
		LifeExtensionPct:    lifeExtensionPct,
		NewWearRateUmHour:   newWearRate,
		OldWearRateUmHour:   oldWearRate,
		ActionSummary:       fmt.Sprintf("添加 %.0fml %s，改善润滑状态", params.AmountML, lub.NameCN),
		MaintenanceCostHint: &costHint,
	}, nil
}

func (vrm *VRMaintenanceManager) ExecuteLubricantAddition(ctx context.Context, params AddLubricantParams) (*models.MaintenanceRecord, error) {
	preview, err := vrm.PreviewLubricantAddition(ctx, params)
	if err != nil {
		return nil, err
	}

	lub, _ := vrm.lubricantsCfg.GetLubricant(params.LubricantCode)

	rec := &models.MaintenanceRecord{
		BearingID:       params.BearingID,
		PerformedAt:     time.Now(),
		MaintenanceType: "add_lubricant",
		Action:          fmt.Sprintf("添加润滑剂: %s (%.0fml)", lub.NameCN, params.AmountML),
		LubricantCode:   &params.LubricantCode,
		LubricantAmount: &params.AmountML,
		WearBeforeUm:    &preview.CurrentWearUm,
		OperatorName:    params.OperatorName,
		Notes:           params.Notes,
		UserSessionID:   params.SessionID,
	}

	err = vrm.saveMaintenanceRecord(ctx, rec)
	if err != nil {
		return nil, fmt.Errorf("保存维护记录失败: %w", err)
	}

	wearAfter := preview.CurrentWearUm * 0.98
	rec.WearAfterUm = &wearAfter
	_ = vrm.db.UpsertBearingLubricationStatus(ctx, params.BearingID, params.LubricantCode, params.AmountML, params.OperatorName)

	return rec, nil
}

func (vrm *VRMaintenanceManager) GetMaintenanceHistory(ctx context.Context, bearingID, limit int) ([]models.MaintenanceRecord, error) {
	return vrm.db.GetMaintenanceRecords(ctx, bearingID, limit)
}

func (vrm *VRMaintenanceManager) GenerateMaintenancePlan(ctx context.Context, bearingID int) (map[string]interface{}, error) {
	status, err := vrm.db.GetBearingLatestStatusByID(ctx, bearingID)
	if err != nil {
		return nil, err
	}

	bearing, err := vrm.db.GetBearingByID(ctx, bearingID)
	if err != nil {
		return nil, err
	}

	wearPct := 0.0
	if status.TotalWearMicrom != nil {
		wearPct = *status.TotalWearMicrom / bearing.WearLimitMicrom
	}

	rulHours := 0.0
	if status.PredictedRULHours != nil {
		rulHours = *status.PredictedRULHours
	}

	materialCode := vrm.guessMaterialCode(bearing)

	plan := map[string]interface{}{
		"bearing_id":          bearingID,
		"bearing_code":        bearing.BearingCode,
		"current_wear_pct":    wearPct * 100,
		"remaining_life_h":    rulHours,
		"health_status":       status.HealthStatus,
		"recommended_actions": make([]map[string]interface{}, 0),
	}

	actions := make([]map[string]interface{}, 0)

	if wearPct > 0.9 {
		actions = append(actions, map[string]interface{}{
			"priority":  "urgent",
			"type":      "replace_bearing",
			"title":     "紧急更换轴承",
			"detail":    fmt.Sprintf("磨损已达 %.0f%%，建议立即更换为青铜或现代轴承", wearPct*100),
			"materials": vrm.suggestedReplacementMaterials(materialCode, true),
		})
	} else if wearPct > 0.7 {
		actions = append(actions, map[string]interface{}{
			"priority":  "high",
			"type":      "plan_replacement",
			"title":     "计划更换轴承",
			"detail":    fmt.Sprintf("磨损已达 %.0f%%，建议1个月内安排更换", wearPct*100),
			"materials": vrm.suggestedReplacementMaterials(materialCode, false),
		})
	}

	lubFreq, hasFreq := vrm.lubricantsCfg.RecommendedLubricationFreq[materialCode]
	if !hasFreq {
		lubFreq = 168
	}
	actions = append(actions, map[string]interface{}{
		"priority":              "routine",
		"type":                  "add_lubricant",
		"title":                 "定期添加润滑剂",
		"detail":                fmt.Sprintf("建议每 %.0f 小时添加一次润滑剂（约 %.0f 天）", lubFreq, lubFreq/24),
		"recommended_lubricants": vrm.suggestedLubricants(materialCode),
		"frequency_hours":       lubFreq,
	})

	if status.Temperature != nil && *status.Temperature > 65 {
		actions = append(actions, map[string]interface{}{
			"priority": "high",
			"type":     "check_overheat",
			"title":    "检查过热原因",
			"detail":   fmt.Sprintf("当前温度 %.1f°C 偏高，可能润滑不足或载荷过大", *status.Temperature),
		})
	}

	plan["recommended_actions"] = actions
	return plan, nil
}

func (vrm *VRMaintenanceManager) saveMaintenanceRecord(ctx context.Context, rec *models.MaintenanceRecord) error {
	id, err := vrm.db.InsertMaintenanceRecord(ctx, rec)
	if err != nil {
		return err
	}
	rec.ID = id
	return nil
}

func (vrm *VRMaintenanceManager) guessMaterialCode(bearing *models.Bearing) string {
	material := bearing.Material
	switch {
	case containsAny(material, "青铜", "铜", "bronze", "Cu"):
		return "bronze_ancient"
	case containsAny(material, "铸铁", "铁", "iron", "cast"):
		return "cast_iron_ancient"
	case containsAny(material, "青冈", "铁栎", "ironbark"):
		return "wood_ironbark"
	case containsAny(material, "橡木", "枣木", "硬木", "oak", "wood"):
		return "wood_oak"
	case containsAny(material, "包铜", "铁皮", "复合", "composite", "wrap"):
		return "wood_wrapped_copper"
	case containsAny(material, "球轴承", "滚动", "GCr15", "ball", "rolling"):
		return "modern_ball_bearing"
	case containsAny(material, "巴氏", "babbitt", "现代滑动"):
		return "modern_bushing_babbit"
	default:
		return "bronze_ancient"
	}
}

func (vrm *VRMaintenanceManager) guessMaterialFromBearing(bearing *models.Bearing) string {
	code := vrm.guessMaterialCode(bearing)
	if mat, ok := vrm.materialsCfg.GetMaterial(code); ok {
		return mat.NameCN
	}
	return bearing.Material
}

func (vrm *VRMaintenanceManager) suggestedReplacementMaterials(currentCode string, urgent bool) []map[string]interface{} {
	results := make([]map[string]interface{}, 0)

	ancientOptions := []string{"bronze_ancient", "wood_wrapped_copper", "wood_ironbark"}
	if urgent {
		ancientOptions = []string{"modern_bushing_babbit", "modern_roller_bearing", "bronze_ancient"}
	}

	for _, code := range ancientOptions {
		if mat, ok := vrm.materialsCfg.GetMaterial(code); ok {
			results = append(results, map[string]interface{}{
				"code":        mat.Code,
				"name":        mat.NameCN,
				"era":         mat.Era,
				"wear_factor": mat.WearResistanceFactor,
				"hardness_hv": mat.HardnessHVNominal,
				"historical":  mat.HistoricalNote,
			})
		}
	}
	return results
}

func (vrm *VRMaintenanceManager) suggestedLubricants(materialCode string) []map[string]interface{} {
	results := make([]map[string]interface{}, 0)

	mat, isMat := vrm.materialsCfg.GetMaterial(materialCode)
	era := "ancient"
	if isMat && mat.Era == "modern" {
		era = "modern"
	}

	priorityCodes := []string{}
	if era == "ancient" {
		priorityCodes = []string{"vegetable_tung", "vegetable_sesame", "animal_beef_tallow", "animal_lard"}
	} else {
		priorityCodes = []string{"mineral_synthetic_pao", "mineral_modern_additive", "mineral_paraffin"}
	}

	for _, code := range priorityCodes {
		if lub, ok := vrm.lubricantsCfg.GetLubricant(code); ok {
			results = append(results, map[string]interface{}{
				"code":           lub.Code,
				"name":           lub.NameCN,
				"category":       lub.Category,
				"wear_reduction": lub.WearReductionRatio * 100,
				"viscosity_40c":  lub.Viscosity40C,
				"life_hours":     lub.MaxLubricationLifeHours,
				"historical":     lub.HistoricalNote,
			})
		}
	}
	return results
}

func (vrm *VRMaintenanceManager) generateReplacementCostHint(materialCode string) string {
	costs := map[string]string{
		"wood_oak":              "成本很低：橡木约 20-50 元/根，木工半天可加工完成",
		"wood_ironbark":         "成本较低：铁栎约 80-150 元/根，需专业木工",
		"wood_wrapped_copper":   "成本中等：木胎+铜皮约 200-500 元，需金工协作",
		"bronze_ancient":        "成本较高：青铜铸件约 500-1500 元，需铸造加工",
		"cast_iron_ancient":     "成本中高：铸铁件约 300-800 元",
		"modern_bushing_babbit": "成本高：现代巴氏合金轴瓦约 1500-3000 元，精密加工",
		"modern_ball_bearing":   "成本很高：标准滚动轴承约 800-2000 元，需改轴尺寸",
		"modern_roller_bearing": "成本很高：重型滚子轴承约 2000-5000 元",
	}
	if hint, ok := costs[materialCode]; ok {
		return hint
	}
	return "成本取决于材料选择和加工工艺"
}

func (vrm *VRMaintenanceManager) generateLubricantCostHint(lubricantCode string, amountML float64) string {
	costsPerLiter := map[string]float64{
		"vegetable_tung":          80.0,
		"vegetable_rape":          15.0,
		"vegetable_sesame":        40.0,
		"animal_lard":             25.0,
		"animal_beef_tallow":      30.0,
		"animal_whale":            500.0,
		"mineral_paraffin":        25.0,
		"mineral_synthetic_pao":   150.0,
		"mineral_modern_additive": 80.0,
	}
	unitPrice, hasPrice := costsPerLiter[lubricantCode]
	if !hasPrice {
		return "成本取决于润滑剂类型和采购渠道"
	}
	totalCost := unitPrice * amountML / 1000.0
	lubName := lubricantCode
	if lub, ok := vrm.lubricantsCfg.GetLubricant(lubricantCode); ok {
		lubName = lub.NameCN
	}
	return fmt.Sprintf("%s %.0fml 约 %.1f 元 (%.0f 元/升)", lubName, amountML, totalCost, unitPrice)
}

func (vrm *VRMaintenanceManager) reductionPct(oldVal, newVal float64) float64 {
	if oldVal <= 0 {
		return 0
	}
	return (oldVal - newVal) / oldVal * 100.0
}

type SmartRecommendation struct {
	RecommendedMaterial       string
	RecommendedMaterialName   string
	RecommendedLubricant      string
	RecommendedLubricantName  string
	RecommendedLubricantML    float64
	Reasoning                 string
	EstimatedLifeHours        float64
	EstimatedCostCNY          float64
}

func (vrm *VRMaintenanceManager) SmartRecommend(ctx context.Context, bearingID int) (*SmartRecommendation, error) {
	bearing, err := vrm.db.GetBearingByID(ctx, bearingID)
	if err != nil {
		return nil, fmt.Errorf("获取轴承信息失败: %w", err)
	}

	status, err := vrm.db.GetBearingLatestStatusByID(ctx, bearingID)
	if err != nil {
		return nil, fmt.Errorf("获取轴承状态失败: %w", err)
	}

	loadN := 5000.0
	if status.RadialLoad != nil {
		loadN = *status.RadialLoad
	}
	speedRPM := 15.0
	if status.RotationalSpeed != nil {
		speedRPM = *status.RotationalSpeed
	}
	tempC := 40.0
	if status.Temperature != nil {
		tempC = *status.Temperature
	}

	currentWear := 0.0
	if status.TotalWearMicrom != nil {
		currentWear = *status.TotalWearMicrom
	}
	wearPct := currentWear / bearing.WearLimitMicrom * 100.0

	materialCode := vrm.guessMaterialCode(bearing)
	allMaterials := vrm.suggestedReplacementMaterials(materialCode, wearPct > 70)
	allLubricants := vrm.suggestedLubricants(materialCode)

	bestMaterialCode := ""
	bestLife := 0.0
	bestMaterialName := ""
	for _, m := range allMaterials {
		code, _ := m["code"].(string)
		name, _ := m["name"].(string)
		codes := []string{code}
		compareResult := vrm.mc.CompareMaterials(bearing, codes, loadN, speedRPM, tempC, 8760.0)
		if len(compareResult.Items) > 0 && compareResult.Items[0].PredictedLifeHours > bestLife {
			bestLife = compareResult.Items[0].PredictedLifeHours
			bestMaterialCode = code
			bestMaterialName = name
		}
	}

	bestLubricantCode := "vegetable_tung"
	bestLubricantName := "桐油"
	bestLubricantML := 100.0
	if len(allLubricants) > 0 {
		bestLubricantCode, _ = allLubricants[0]["code"].(string)
		bestLubricantName, _ = allLubricants[0]["name"].(string)
	}

	reasoning := fmt.Sprintf(
		"当前磨损 %.1f%% (%.1fμm)。推荐更换为 %s，配合 %s %.0fml 润滑。预计寿命延长 %.0f 小时。",
		wearPct, currentWear, bestMaterialName, bestLubricantName, bestLubricantML, bestLife,
	)

	costMap := map[string]float64{
		"wood_oak":              50.0,
		"wood_ironbark":         120.0,
		"wood_wrapped_copper":   350.0,
		"bronze_ancient":        1000.0,
		"cast_iron_ancient":     550.0,
		"modern_bushing_babbit": 2250.0,
		"modern_ball_bearing":   1400.0,
		"modern_roller_bearing": 3500.0,
	}
	lubCostPerLiter := map[string]float64{
		"vegetable_tung":     80.0,
		"vegetable_rape":     15.0,
		"vegetable_sesame":   40.0,
		"animal_lard":        25.0,
		"animal_beef_tallow": 30.0,
	}

	estCost := costMap[bestMaterialCode]
	if lubCost, ok := lubCostPerLiter[bestLubricantCode]; ok {
		estCost += lubCost * bestLubricantML / 1000.0
	}

	return &SmartRecommendation{
		RecommendedMaterial:       bestMaterialCode,
		RecommendedMaterialName:   bestMaterialName,
		RecommendedLubricant:      bestLubricantCode,
		RecommendedLubricantName:  bestLubricantName,
		RecommendedLubricantML:    bestLubricantML,
		Reasoning:                 reasoning,
		EstimatedLifeHours:        bestLife,
		EstimatedCostCNY:          estCost,
	}, nil
}

func (vrm *VRMaintenanceManager) OneClickReplaceBearing(ctx context.Context, bearingID int, operatorName *string) (*models.MaintenanceRecord, error) {
	recommendation, err := vrm.SmartRecommend(ctx, bearingID)
	if err != nil {
		return nil, fmt.Errorf("智能推荐失败: %w", err)
	}

	params := ReplaceBearingParams{
		BearingID:       bearingID,
		NewMaterialCode: recommendation.RecommendedMaterial,
		OperatorName:    operatorName,
		Notes:           &recommendation.Reasoning,
	}

	return vrm.ExecuteBearingReplacement(ctx, params)
}

func (vrm *VRMaintenanceManager) OneClickAddLubricant(ctx context.Context, bearingID int, operatorName *string) (*models.MaintenanceRecord, error) {
	recommendation, err := vrm.SmartRecommend(ctx, bearingID)
	if err != nil {
		return nil, fmt.Errorf("智能推荐失败: %w", err)
	}

	params := AddLubricantParams{
		BearingID:     bearingID,
		LubricantCode: recommendation.RecommendedLubricant,
		AmountML:      recommendation.RecommendedLubricantML,
		OperatorName:  operatorName,
	}

	return vrm.ExecuteLubricantAddition(ctx, params)
}

func containsAny(s string, keywords ...string) bool {
	sLower := toLower(s)
	for _, kw := range keywords {
		if contains(sLower, toLower(kw)) {
			return true
		}
	}
	return false
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || (len(s) > 0 && indexOf(s, sub) >= 0))
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func toLower(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c = c + ('a' - 'A')
		}
		result[i] = c
	}
	return string(result)
}
