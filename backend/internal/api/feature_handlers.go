package api

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"noria-bearing-system/internal/database"
	"noria-bearing-system/internal/models"
	"noria-bearing-system/internal/modules/analysis"
	"noria-bearing-system/internal/modules/maintenance"
)

type FeatureHandler struct {
	engine        *analysis.ComparisonEngine
	maintenance   *maintenance.MaintenanceManager
}

func NewFeatureHandler() *FeatureHandler {
	return &FeatureHandler{
		engine:      analysis.NewComparisonEngine(),
		maintenance: maintenance.NewMaintenanceManager(database.Instance),
	}
}

func (fh *FeatureHandler) ListBearingMaterials(c *gin.Context) {
	eraFilter := c.Query("era")
	categoryFilter := c.Query("category")

	var materials interface{}
	if eraFilter != "" {
		materials = fh.engine.GetMaterialsByEra(eraFilter)
	} else if categoryFilter != "" {
		materials = fh.engine.GetAllMaterials()
	} else {
		materials = fh.engine.GetAllMaterials()
	}

	c.JSON(http.StatusOK, gin.H{
		"count": len(materials.([]interface{})),
		"data":  materials,
	})
}

func (fh *FeatureHandler) ListLubricants(c *gin.Context) {
	categoryFilter := c.Query("category")
	eraFilter := c.Query("era")

	var results interface{}
	if categoryFilter != "" {
		results = fh.engine.GetLubricantsByCategory(categoryFilter)
	} else if eraFilter != "" {
		results = fh.engine.GetAllLubricants()
	} else {
		results = fh.engine.GetAllLubricants()
	}

	c.JSON(http.StatusOK, gin.H{
		"count": len(results.([]interface{})),
		"data":  results,
	})
}

type compareMaterialsRequest struct {
	BearingID       int      `json:"bearing_id" binding:"required"`
	MaterialCodes   []string `json:"material_codes" binding:"required,min=2"`
	ReferenceLoadN  *float64 `json:"reference_load_n"`
	ReferenceSpeed  *float64 `json:"reference_speed_rpm"`
	ReferenceTemp   *float64 `json:"reference_temp_celsius"`
	SimulationHours *float64 `json:"simulation_hours"`
	SaveReport      bool     `json:"save_report"`
	SessionID       *string  `json:"user_session_id"`
	Title           *string  `json:"title"`
}

func (fh *FeatureHandler) CompareMaterials(c *gin.Context) {
	var req compareMaterialsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}

	ctx := context.Background()
	bearing, err := database.Instance.GetBearingByID(ctx, req.BearingID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取轴承失败: " + err.Error()})
		return
	}

	loadN := 5000.0
	speedRPM := 15.0
	tempC := 40.0
	simHours := 8760.0

	statuses, _ := database.Instance.GetAllBearingLatestStatus(ctx)
	for _, s := range statuses {
		if s.BearingID == req.BearingID {
			if s.RadialLoad != nil {
				loadN = *s.RadialLoad
			}
			if s.RotationalSpeed != nil {
				speedRPM = *s.RotationalSpeed
			}
			if s.Temperature != nil {
				tempC = *s.Temperature
			}
			break
		}
	}

	if req.ReferenceLoadN != nil {
		loadN = *req.ReferenceLoadN
	}
	if req.ReferenceSpeed != nil {
		speedRPM = *req.ReferenceSpeed
	}
	if req.ReferenceTemp != nil {
		tempC = *req.ReferenceTemp
	}
	if req.SimulationHours != nil {
		simHours = *req.SimulationHours
	}

	result := fh.engine.CompareMaterials(bearing, req.MaterialCodes, loadN, speedRPM, tempC, simHours)

	if req.SaveReport {
		paramsJSON, _ := jsonMarshal(req)
		resultJSON, _ := jsonMarshal(result)
		bearingID := req.BearingID
		_, _ = database.Instance.InsertComparisonReport(ctx, "material_comparison", &bearingID, paramsJSON, resultJSON, req.SessionID, req.Title)
	}

	c.JSON(http.StatusOK, result)
}

type compareLubricantsRequest struct {
	BearingID       int      `json:"bearing_id" binding:"required"`
	MaterialCode    *string  `json:"material_code"`
	LubricantCodes  []string `json:"lubricant_codes" binding:"required,min=2"`
	ReferenceLoadN  *float64 `json:"reference_load_n"`
	ReferenceSpeed  *float64 `json:"reference_speed_rpm"`
	ReferenceTemp   *float64 `json:"reference_temp_celsius"`
	SimulationHours *float64 `json:"simulation_hours"`
	SaveReport      bool     `json:"save_report"`
	SessionID       *string  `json:"user_session_id"`
	Title           *string  `json:"title"`
}

func (fh *FeatureHandler) CompareLubricants(c *gin.Context) {
	var req compareLubricantsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}

	ctx := context.Background()
	bearing, err := database.Instance.GetBearingByID(ctx, req.BearingID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取轴承失败: " + err.Error()})
		return
	}

	materialCode := guessMaterialCode(bearing)
	if req.MaterialCode != nil {
		materialCode = *req.MaterialCode
	}

	loadN := 5000.0
	speedRPM := 15.0
	tempC := 40.0
	simHours := 8760.0

	statuses, _ := database.Instance.GetAllBearingLatestStatus(ctx)
	for _, s := range statuses {
		if s.BearingID == req.BearingID {
			if s.RadialLoad != nil {
				loadN = *s.RadialLoad
			}
			if s.RotationalSpeed != nil {
				speedRPM = *s.RotationalSpeed
			}
			if s.Temperature != nil {
				tempC = *s.Temperature
			}
			break
		}
	}

	if req.ReferenceLoadN != nil {
		loadN = *req.ReferenceLoadN
	}
	if req.ReferenceSpeed != nil {
		speedRPM = *req.ReferenceSpeed
	}
	if req.ReferenceTemp != nil {
		tempC = *req.ReferenceTemp
	}
	if req.SimulationHours != nil {
		simHours = *req.SimulationHours
	}

	result := fh.engine.CompareLubricants(bearing, materialCode, req.LubricantCodes, loadN, speedRPM, tempC, simHours)

	if req.SaveReport {
		paramsJSON, _ := jsonMarshal(req)
		resultJSON, _ := jsonMarshal(result)
		bearingID := req.BearingID
		_, _ = database.Instance.InsertComparisonReport(ctx, "lubricant_comparison", &bearingID, paramsJSON, resultJSON, req.SessionID, req.Title)
	}

	c.JSON(http.StatusOK, result)
}

type crossEraComparisonRequest struct {
	BearingDiameter *float64 `json:"bearing_diameter_mm"`
	BearingWidth    *float64 `json:"bearing_width_mm"`
	ReferenceLoadN  *float64 `json:"reference_load_n"`
	ReferenceSpeed  *float64 `json:"reference_speed_rpm"`
	ReferenceTemp   *float64 `json:"reference_temp_celsius"`
	SimulationHours *float64 `json:"simulation_hours"`
	SaveReport      bool     `json:"save_report"`
	SessionID       *string  `json:"user_session_id"`
	Title           *string  `json:"title"`
}

func (fh *FeatureHandler) CrossEraComparison(c *gin.Context) {
	var req crossEraComparisonRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}

	diameterMM := 150.0
	widthMM := 80.0
	loadN := 5000.0
	speedRPM := 15.0
	tempC := 40.0
	simHours := 8760.0

	if req.BearingDiameter != nil {
		diameterMM = *req.BearingDiameter
	}
	if req.BearingWidth != nil {
		widthMM = *req.BearingWidth
	}
	if req.ReferenceLoadN != nil {
		loadN = *req.ReferenceLoadN
	}
	if req.ReferenceSpeed != nil {
		speedRPM = *req.ReferenceSpeed
	}
	if req.ReferenceTemp != nil {
		tempC = *req.ReferenceTemp
	}
	if req.SimulationHours != nil {
		simHours = *req.SimulationHours
	}

	result := fh.engine.CrossEraComparison(diameterMM, widthMM, loadN, speedRPM, tempC, simHours)

	if req.SaveReport {
		ctx := context.Background()
		paramsJSON, _ := jsonMarshal(req)
		resultJSON, _ := jsonMarshal(result)
		_, _ = database.Instance.InsertComparisonReport(ctx, "cross_era", nil, paramsJSON, resultJSON, req.SessionID, req.Title)
	}

	c.JSON(http.StatusOK, result)
}

type previewReplacementRequest struct {
	BearingID       int     `json:"bearing_id" binding:"required"`
	NewMaterialCode string  `json:"new_material_code" binding:"required"`
	OperatorName    *string `json:"operator_name"`
	SessionID       *string `json:"session_id"`
}

type executeReplacementRequest struct {
	BearingID       int     `json:"bearing_id" binding:"required"`
	NewMaterialCode string  `json:"new_material_code" binding:"required"`
	OperatorName    *string `json:"operator_name"`
	SessionID       *string `json:"session_id"`
	Notes           *string `json:"notes"`
}

func (fh *FeatureHandler) PreviewBearingReplacement(c *gin.Context) {
	var req previewReplacementRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}

	ctx := context.Background()
	params := maintenance.ReplaceBearingParams{
		BearingID:       req.BearingID,
		NewMaterialCode: req.NewMaterialCode,
		OperatorName:    req.OperatorName,
		SessionID:       req.SessionID,
	}

	preview, err := fh.maintenance.PreviewBearingReplacement(ctx, params)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, preview)
}

func (fh *FeatureHandler) ExecuteBearingReplacement(c *gin.Context) {
	var req executeReplacementRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}

	ctx := context.Background()
	params := maintenance.ReplaceBearingParams{
		BearingID:       req.BearingID,
		NewMaterialCode: req.NewMaterialCode,
		OperatorName:    req.OperatorName,
		SessionID:       req.SessionID,
		Notes:           req.Notes,
	}

	record, err := fh.maintenance.ExecuteBearingReplacement(ctx, params)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "轴承更换完成",
		"record":  record,
	})
}

type previewLubricantRequest struct {
	BearingID       int     `json:"bearing_id" binding:"required"`
	LubricantCode   string  `json:"lubricant_code" binding:"required"`
	AmountML        float64 `json:"lubricant_amount_ml" binding:"required,gt=0"`
	OperatorName    *string `json:"operator_name"`
	SessionID       *string `json:"session_id"`
}

type executeLubricantRequest struct {
	BearingID       int     `json:"bearing_id" binding:"required"`
	LubricantCode   string  `json:"lubricant_code" binding:"required"`
	AmountML        float64 `json:"lubricant_amount_ml" binding:"required,gt=0"`
	OperatorName    *string `json:"operator_name"`
	SessionID       *string `json:"session_id"`
	Notes           *string `json:"notes"`
}

func (fh *FeatureHandler) PreviewLubricantAddition(c *gin.Context) {
	var req previewLubricantRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}

	ctx := context.Background()
	params := maintenance.AddLubricantParams{
		BearingID:     req.BearingID,
		LubricantCode: req.LubricantCode,
		AmountML:      req.AmountML,
		OperatorName:  req.OperatorName,
		SessionID:     req.SessionID,
	}

	preview, err := fh.maintenance.PreviewLubricantAddition(ctx, params)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, preview)
}

func (fh *FeatureHandler) ExecuteLubricantAddition(c *gin.Context) {
	var req executeLubricantRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}

	ctx := context.Background()
	params := maintenance.AddLubricantParams{
		BearingID:     req.BearingID,
		LubricantCode: req.LubricantCode,
		AmountML:      req.AmountML,
		OperatorName:  req.OperatorName,
		SessionID:     req.SessionID,
		Notes:         req.Notes,
	}

	record, err := fh.maintenance.ExecuteLubricantAddition(ctx, params)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "润滑剂添加完成",
		"record":  record,
	})
}

func (fh *FeatureHandler) GetMaintenanceHistory(c *gin.Context) {
	ctx := context.Background()
	bearingIDStr := c.DefaultQuery("bearing_id", "0")
	limitStr := c.DefaultQuery("limit", "50")

	bearingID, _ := strconv.Atoi(bearingIDStr)
	limit, _ := strconv.Atoi(limitStr)
	if limit <= 0 {
		limit = 50
	}

	records, err := fh.maintenance.GetMaintenanceHistory(ctx, bearingID, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"count": len(records),
		"data":  records,
	})
}

func (fh *FeatureHandler) GetMaintenancePlan(c *gin.Context) {
	ctx := context.Background()
	bearingIDStr := c.Param("bearing_id")
	bearingID, err := strconv.Atoi(bearingIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的轴承ID"})
		return
	}

	plan, err := fh.maintenance.GenerateMaintenancePlan(ctx, bearingID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, plan)
}

func (fh *FeatureHandler) GetMaterialReferenceData(c *gin.Context) {
	result := fh.engine.GetAllMaterials()
	c.JSON(http.StatusOK, gin.H{
		"count":   len(result),
		"columns": []string{"材料名称", "时代", "类型", "硬度HV", "弹性模量GPa", "耐磨性系数", "历史背景"},
		"data":    result,
	})
}

func (fh *FeatureHandler) GetLubricantReferenceData(c *gin.Context) {
	result := fh.engine.GetAllLubricants()
	c.JSON(http.StatusOK, gin.H{
		"count":   len(result),
		"columns": []string{"名称", "类型", "时代", "粘度40°C", "粘度指数", "减磨率%", "推荐寿命h", "历史背景"},
		"data":    result,
	})
}

func guessMaterialCode(bearing *models.Bearing) string {
	material := bearing.Material
	switch {
	case containsCI(material, "青铜") || containsCI(material, "铜") || containsCI(material, "bronze"):
		return "bronze_ancient"
	case containsCI(material, "铸铁") || containsCI(material, "铁"):
		return "cast_iron_ancient"
	case containsCI(material, "青冈") || containsCI(material, "铁栎"):
		return "wood_ironbark"
	case containsCI(material, "橡木") || containsCI(material, "枣木") || containsCI(material, "硬木") || containsCI(material, "wood"):
		return "wood_oak"
	case containsCI(material, "包铜") || containsCI(material, "铁皮") || containsCI(material, "复合"):
		return "wood_wrapped_copper"
	case containsCI(material, "滚动") || containsCI(material, "球轴承") || containsCI(material, "GCr15"):
		return "modern_ball_bearing"
	case containsCI(material, "巴氏") || containsCI(material, "现代滑动"):
		return "modern_bushing_babbit"
	default:
		return "bronze_ancient"
	}
}

func containsCI(s, substr string) bool {
	sLower := toLowerASCII(s)
	subLower := toLowerASCII(substr)
	return strings.Contains(sLower, subLower)
}

func toLowerASCII(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		b := s[i]
		if b >= 'A' && b <= 'Z' {
			b = b + 32
		}
		result[i] = b
	}
	return string(result)
}

func jsonMarshal(v interface{}) interface{} {
	return v
}
