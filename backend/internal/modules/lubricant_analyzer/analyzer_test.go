package lubricant_analyzer

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"noria-bearing-system/internal/config"
	"noria-bearing-system/internal/models"
)

func setupLubTestConfig(t *testing.T) {
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

func makeLubTestBearing() *models.Bearing {
	return &models.Bearing{
		ID:              1,
		BearingCode:     "TEST-001",
		NoriaID:         1,
		InnerDiameter:   150.0,
		OuterDiameter:   225.0,
		Width:           80.0,
		Material:        "青铜",
		WearLimitMicrom: 500.0,
		Position:        "上轴承",
	}
}

func TestNewLubricantAnalyzer(t *testing.T) {
	setupLubTestConfig(t)
	la := NewLubricantAnalyzer()
	defer la.Close()

	if la == nil {
		t.Fatal("NewLubricantAnalyzer returned nil")
	}
	if la.materials == nil {
		t.Error("materials config should not be nil")
	}
	if la.lubricants == nil {
		t.Error("lubricants config should not be nil")
	}
	if la.calculator == nil {
		t.Error("wearcalc calculator should not be nil")
	}
}

func TestGetAllLubricants(t *testing.T) {
	setupLubTestConfig(t)
	la := NewLubricantAnalyzer()
	defer la.Close()

	lubs := la.GetAllLubricants()
	if len(lubs) == 0 {
		t.Fatal("GetAllLubricants should return non-empty list")
	}

	catCount := map[string]int{}
	for _, l := range lubs {
		if l.Code == "" {
			t.Error("Lubricant code should not be empty")
		}
		if l.NameCN == "" {
			t.Errorf("Lubricant %s has no Chinese name", l.Code)
		}
		catCount[l.Category]++
	}
	if catCount["vegetable"] < 1 {
		t.Errorf("Expected at least 1 vegetable lubricant, got %d", catCount["vegetable"])
	}
	if catCount["animal"] < 1 {
		t.Errorf("Expected at least 1 animal lubricant, got %d", catCount["animal"])
	}
}

func TestGetLubricantsByCategory(t *testing.T) {
	setupLubTestConfig(t)
	la := NewLubricantAnalyzer()
	defer la.Close()

	vegetable := la.GetLubricantsByCategory("vegetable")
	if len(vegetable) == 0 {
		t.Fatal("Expected vegetable lubricants")
	}
	for _, l := range vegetable {
		if l.Category != "vegetable" {
			t.Errorf("Expected category=vegetable, got %s for %s", l.Category, l.Code)
		}
	}

	mineral := la.GetLubricantsByCategory("mineral")
	if len(mineral) == 0 {
		t.Fatal("Expected mineral lubricants")
	}
	for _, l := range mineral {
		if l.Category != "mineral" {
			t.Errorf("Expected category=mineral, got %s for %s", l.Category, l.Code)
		}
	}
}

func TestCompareLubricants_BasicResult(t *testing.T) {
	setupLubTestConfig(t)
	la := NewLubricantAnalyzer()
	defer la.Close()

	bearing := makeLubTestBearing()
	codes := []string{"vegetable_tung", "animal_beef_tallow"}

	result := la.CompareLubricants(bearing, "bronze_ancient", codes, 5000.0, 15.0, 40.0, 8760.0)
	if result == nil {
		t.Fatal("CompareLubricants returned nil")
	}
	if len(result.Items) < 2 {
		t.Errorf("Expected at least 2 items, got %d", len(result.Items))
	}
	if result.BaseMaterial != "bronze_ancient" {
		t.Errorf("BaseMaterial should be bronze_ancient, got %s", result.BaseMaterial)
	}
}

func TestCompareLubricants_WearReductionVsDry(t *testing.T) {
	setupLubTestConfig(t)
	la := NewLubricantAnalyzer()
	defer la.Close()

	bearing := makeLubTestBearing()
	codes := []string{"vegetable_tung"}

	result := la.CompareLubricants(bearing, "bronze_ancient", codes, 5000.0, 15.0, 40.0, 8760.0)
	if len(result.Items) == 0 {
		t.Fatal("Expected at least 1 item")
	}

	item := result.Items[0]
	if item.WearReductionVsDry < 0 {
		t.Errorf("WearReductionVsDry should be non-negative, got %f", item.WearReductionVsDry)
	}
	if item.LifeExtensionVsDry < 0 {
		t.Errorf("LifeExtensionVsDry should be non-negative, got %f", item.LifeExtensionVsDry)
	}
}

func TestCompareLubricants_InvalidCode(t *testing.T) {
	setupLubTestConfig(t)
	la := NewLubricantAnalyzer()
	defer la.Close()

	bearing := makeLubTestBearing()
	codes := []string{"nonexistent_lub_xyz", "vegetable_tung"}

	result := la.CompareLubricants(bearing, "bronze_ancient", codes, 5000.0, 15.0, 40.0, 8760.0)
	if result == nil {
		t.Fatal("CompareLubricants should not return nil for invalid codes")
	}
	if len(result.Items) != 1 {
		t.Errorf("Expected 1 valid item (invalid code skipped), got %d", len(result.Items))
	}
}

func TestCompareLubricants_RankOrdering(t *testing.T) {
	setupLubTestConfig(t)
	la := NewLubricantAnalyzer()
	defer la.Close()

	bearing := makeLubTestBearing()
	codes := []string{"vegetable_tung", "animal_beef_tallow", "vegetable_sesame"}

	result := la.CompareLubricants(bearing, "bronze_ancient", codes, 5000.0, 15.0, 40.0, 8760.0)

	prevLife := 1e18
	for idx, item := range result.Items {
		if item.Rank != idx+1 {
			t.Errorf("Rank mismatch at index %d: expected %d, got %d", idx, idx+1, item.Rank)
		}
		if item.PredictedLifeHours > prevLife {
			t.Errorf("Items not sorted by life desc: %f after %f", item.PredictedLifeHours, prevLife)
		}
		prevLife = item.PredictedLifeHours
	}

	if len(result.Items) > 0 {
		best := result.Items[0]
		if best.LifeRatioVsBest != 1.0 {
			t.Errorf("Best item LifeRatioVsBest should be 1.0, got %f", best.LifeRatioVsBest)
		}
	}
}

func TestCompareLubricants_RegimeMapping(t *testing.T) {
	setupLubTestConfig(t)
	la := NewLubricantAnalyzer()
	defer la.Close()

	bearing := makeLubTestBearing()
	codes := []string{"vegetable_tung"}

	result := la.CompareLubricants(bearing, "bronze_ancient", codes, 5000.0, 15.0, 40.0, 8760.0)
	if len(result.Items) == 0 {
		t.Fatal("Expected at least 1 item")
	}

	validRegimes := map[string]bool{
		"全膜弹流润滑": true,
		"混合润滑":   true,
		"边界润滑":   true,
	}
	if !validRegimes[result.Items[0].LubricationRegime] {
		t.Errorf("Invalid lubrication regime: %s", result.Items[0].LubricationRegime)
	}
}
