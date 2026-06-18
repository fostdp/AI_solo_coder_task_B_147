package era_comparator

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"noria-bearing-system/internal/config"
)

func setupEraTestConfig(t *testing.T) {
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
}

func TestNewEraComparator(t *testing.T) {
	setupEraTestConfig(t)
	ec := NewEraComparator()
	defer ec.Close()

	if ec == nil {
		t.Fatal("NewEraComparator returned nil")
	}
	if ec.materials == nil {
		t.Error("materials config should not be nil")
	}
	if ec.lubricants == nil {
		t.Error("lubricants config should not be nil")
	}
	if ec.calculator == nil {
		t.Error("wearcalc calculator should not be nil")
	}
	if ec.mc == nil {
		t.Error("material_comparator should not be nil")
	}
}

func TestCrossEraComparison_BasicResult(t *testing.T) {
	setupEraTestConfig(t)
	ec := NewEraComparator()
	defer ec.Close()

	result := ec.CrossEraComparison(150.0, 80.0, 5000.0, 15.0, 40.0, 8760.0)
	if result == nil {
		t.Fatal("CrossEraComparison returned nil")
	}
	if result.ReferenceLoad != 5000.0 {
		t.Errorf("ReferenceLoad mismatch: expected 5000, got %f", result.ReferenceLoad)
	}
	if result.ReferenceSpeed != 15.0 {
		t.Errorf("ReferenceSpeed mismatch: expected 15, got %f", result.ReferenceSpeed)
	}
	if result.BearingDiameter != 150.0 {
		t.Errorf("BearingDiameter mismatch: expected 150, got %f", result.BearingDiameter)
	}
}

func TestCrossEraComparison_AncientBestExists(t *testing.T) {
	setupEraTestConfig(t)
	ec := NewEraComparator()
	defer ec.Close()

	result := ec.CrossEraComparison(150.0, 80.0, 5000.0, 15.0, 40.0, 8760.0)
	if result.AncientBest == nil {
		t.Fatal("AncientBest should not be nil")
	}
	if result.AncientBest.Era != "ancient" {
		t.Errorf("AncientBest era should be 'ancient', got '%s'", result.AncientBest.Era)
	}
	if result.AncientBest.PredictedLifeHours <= 0 {
		t.Errorf("AncientBest PredictedLifeHours should be positive, got %f", result.AncientBest.PredictedLifeHours)
	}
}

func TestCrossEraComparison_ModernBestExists(t *testing.T) {
	setupEraTestConfig(t)
	ec := NewEraComparator()
	defer ec.Close()

	result := ec.CrossEraComparison(150.0, 80.0, 5000.0, 15.0, 40.0, 8760.0)
	if result.ModernBest == nil {
		t.Fatal("ModernBest should not be nil")
	}
	if result.ModernBest.Era != "modern" {
		t.Errorf("ModernBest era should be 'modern', got '%s'", result.ModernBest.Era)
	}
	if result.ModernBest.PredictedLifeHours <= 0 {
		t.Errorf("ModernBest PredictedLifeHours should be positive, got %f", result.ModernBest.PredictedLifeHours)
	}
}

func TestCrossEraComparison_ModernBetterThanAncient(t *testing.T) {
	setupEraTestConfig(t)
	ec := NewEraComparator()
	defer ec.Close()

	result := ec.CrossEraComparison(150.0, 80.0, 5000.0, 15.0, 40.0, 8760.0)
	if result.LifeImprovementX < 1.0 {
		t.Errorf("Modern life should exceed ancient life, improvementX=%f", result.LifeImprovementX)
	}
	if result.WearReductionPct < 0 {
		t.Errorf("WearReductionPct should be non-negative, got %f", result.WearReductionPct)
	}
}

func TestCrossEraComparison_AllItemsSorted(t *testing.T) {
	setupEraTestConfig(t)
	ec := NewEraComparator()
	defer ec.Close()

	result := ec.CrossEraComparison(150.0, 80.0, 5000.0, 15.0, 40.0, 8760.0)
	if len(result.AllItems) == 0 {
		t.Fatal("AllItems should not be empty")
	}

	prevLife := 1e18
	for idx, item := range result.AllItems {
		if item.EraRank != idx+1 {
			t.Errorf("EraRank mismatch at index %d: expected %d, got %d", idx, idx+1, item.EraRank)
		}
		if item.PredictedLifeHours > prevLife {
			t.Errorf("AllItems not sorted by life desc: %f after %f", item.PredictedLifeHours, prevLife)
		}
		prevLife = item.PredictedLifeHours
	}
}

func TestCrossEraComparison_InsightsGenerated(t *testing.T) {
	setupEraTestConfig(t)
	ec := NewEraComparator()
	defer ec.Close()

	result := ec.CrossEraComparison(150.0, 80.0, 5000.0, 15.0, 40.0, 8760.0)
	if len(result.InsightSummary) == 0 {
		t.Fatal("InsightSummary should not be empty")
	}

	hasContent := false
	for _, ins := range result.InsightSummary {
		if len(ins) > 5 {
			hasContent = true
			break
		}
	}
	if !hasContent {
		t.Error("At least one insight should have meaningful content (>5 chars)")
	}
}

func TestCrossEraComparison_BothErasInAllItems(t *testing.T) {
	setupEraTestConfig(t)
	ec := NewEraComparator()
	defer ec.Close()

	result := ec.CrossEraComparison(150.0, 80.0, 5000.0, 15.0, 40.0, 8760.0)

	eraSet := map[string]bool{}
	for _, item := range result.AllItems {
		eraSet[item.Era] = true
	}
	if !eraSet["ancient"] {
		t.Error("AllItems should contain ancient era items")
	}
	if !eraSet["modern"] {
		t.Error("AllItems should contain modern era items")
	}
}

func TestFormatNumber(t *testing.T) {
	cases := []struct {
		x        float64
		decimals int
		expected string
	}{
		{100.0, 0, "100"},
		{3.14159, 1, "3.1"},
		{2.567, 2, "2.57"},
	}
	for _, c := range cases {
		result := formatNumber(c.x, c.decimals)
		if result != c.expected {
			t.Errorf("formatNumber(%f, %d) = '%s', expected '%s'", c.x, c.decimals, result, c.expected)
		}
	}
}
