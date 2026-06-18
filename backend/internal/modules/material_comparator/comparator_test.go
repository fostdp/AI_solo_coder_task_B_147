package material_comparator

import (
	"testing"

	"noria-bearing-system/internal/config"
	"noria-bearing-system/internal/models"
)

func setupTestConfig(t *testing.T) {
	t.Helper()
	err := config.LoadConfig("../../../config")
	if err != nil {
		t.Logf("Config load skipped (standalone test): %v", err)
	}
}

func makeTestBearing() *models.Bearing {
	return &models.Bearing{
		ID:             1,
		BearingCode:    "TEST-001",
		NoriaID:        1,
		InnerDiameter:  150.0,
		OuterDiameter:  225.0,
		Width:          80.0,
		Material:       "青铜",
		WearLimitMicrom: 500.0,
		Position:       "上轴承",
	}
}

func TestNewMaterialComparator(t *testing.T) {
	setupTestConfig(t)
	mc := NewMaterialComparator()
	defer mc.Close()

	if mc == nil {
		t.Fatal("NewMaterialComparator returned nil")
	}
	if mc.materials == nil {
		t.Error("materials config should not be nil")
	}
	if mc.lubricants == nil {
		t.Error("lubricants config should not be nil")
	}
	if mc.calculator == nil {
		t.Error("wearcalc calculator should not be nil")
	}
}

func TestGetAllMaterials(t *testing.T) {
	setupTestConfig(t)
	mc := NewMaterialComparator()
	defer mc.Close()

	mats := mc.GetAllMaterials()
	if len(mats) == 0 {
		t.Fatal("GetAllMaterials should return non-empty list")
	}

	eraCount := map[string]int{}
	for _, m := range mats {
		if m.Code == "" {
			t.Error("Material code should not be empty")
		}
		if m.NameCN == "" {
			t.Errorf("Material %s has no Chinese name", m.Code)
		}
		eraCount[m.Era]++
	}
	if eraCount["ancient"] < 3 {
		t.Errorf("Expected at least 3 ancient materials, got %d", eraCount["ancient"])
	}
	if eraCount["modern"] < 2 {
		t.Errorf("Expected at least 2 modern materials, got %d", eraCount["modern"])
	}
}

func TestGetMaterialsByEra(t *testing.T) {
	setupTestConfig(t)
	mc := NewMaterialComparator()
	defer mc.Close()

	ancient := mc.GetMaterialsByEra("ancient")
	if len(ancient) == 0 {
		t.Fatal("Expected ancient materials")
	}
	for _, m := range ancient {
		if m.Era != "ancient" {
			t.Errorf("Expected era=ancient, got %s for %s", m.Era, m.Code)
		}
	}

	modern := mc.GetMaterialsByEra("modern")
	if len(modern) == 0 {
		t.Fatal("Expected modern materials")
	}
	for _, m := range modern {
		if m.Era != "modern" {
			t.Errorf("Expected era=modern, got %s for %s", m.Era, m.Code)
		}
	}
}

func TestCompareMaterials_MinimumTwo(t *testing.T) {
	setupTestConfig(t)
	mc := NewMaterialComparator()
	defer mc.Close()

	bearing := makeTestBearing()
	codes := []string{"bronze_ancient", "wood_oak"}

	result := mc.CompareMaterials(bearing, codes, 5000.0, 15.0, 40.0, 8760.0)
	if result == nil {
		t.Fatal("CompareMaterials returned nil")
	}
	if len(result.Items) < 2 {
		t.Errorf("Expected at least 2 items, got %d", len(result.Items))
	}

	for _, item := range result.Items {
		if item.MaterialCode == "" {
			t.Error("Item MaterialCode should not be empty")
		}
		if item.PredictedLifeHours <= 0 {
			t.Errorf("Item %s PredictedLifeHours should be positive, got %f",
				item.MaterialCode, item.PredictedLifeHours)
		}
		if item.WearRateUmPerHour <= 0 {
			t.Errorf("Item %s WearRate should be positive, got %f",
				item.MaterialCode, item.WearRateUmPerHour)
		}
	}
}

func TestCompareMaterials_InvalidCode(t *testing.T) {
	setupTestConfig(t)
	mc := NewMaterialComparator()
	defer mc.Close()

	bearing := makeTestBearing()
	codes := []string{"nonexistent_code_xyz", "bronze_ancient"}

	result := mc.CompareMaterials(bearing, codes, 5000.0, 15.0, 40.0, 8760.0)
	if result == nil {
		t.Fatal("CompareMaterials should not return nil even for invalid codes")
	}
	if len(result.Items) != 1 {
		t.Errorf("Expected 1 valid item (invalid code skipped), got %d", len(result.Items))
	}
}

func TestCompareMaterials_RankOrdering(t *testing.T) {
	setupTestConfig(t)
	mc := NewMaterialComparator()
	defer mc.Close()

	bearing := makeTestBearing()
	codes := []string{"wood_oak", "bronze_ancient", "modern_ball_bearing"}

	result := mc.CompareMaterials(bearing, codes, 5000.0, 15.0, 40.0, 8760.0)

	prevLife := 1e18
	for idx, item := range result.Items {
		if item.Rank != idx+1 {
			t.Errorf("Item rank mismatch: expected %d, got %d", idx+1, item.Rank)
		}
		if item.PredictedLifeHours > prevLife {
			t.Errorf("Items not sorted by life desc: %f after %f",
				item.PredictedLifeHours, prevLife)
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

func TestCompareMaterialsGeneric_Independence(t *testing.T) {
	setupTestConfig(t)
	mc := NewMaterialComparator()
	defer mc.Close()

	ancientLub := mc.getDefaultAncientLubricant()
	codes := []string{"bronze_ancient", "cast_iron_ancient"}

	items := mc.CompareMaterialsGeneric(
		100.0, 150.0, 50.0, codes, ancientLub,
		3000.0, 20.0, 50.0, 8760.0,
	)

	if len(items) != 2 {
		t.Errorf("Expected 2 items, got %d", len(items))
	}

	for _, item := range items {
		if item.PredictedLifeHours <= 0 {
			t.Errorf("%s PredictedLifeHours should be positive", item.MaterialCode)
		}
	}
}

func TestGetDefaultAncientMaterial(t *testing.T) {
	setupTestConfig(t)
	mc := NewMaterialComparator()
	defer mc.Close()

	mat := mc.GetDefaultAncientMaterial()
	if mat.Code == "" {
		t.Error("Default ancient material should exist")
	}
	if mat.Era != "ancient" {
		t.Errorf("Default material should be ancient era, got %s", mat.Era)
	}
}
