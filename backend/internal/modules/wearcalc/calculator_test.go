package wearcalc

import (
	"math"
	"testing"
)

func TestSimulateWear_BronzeNormal(t *testing.T) {
	wc := NewWearCalculator(2)
	defer wc.Close()

	in := SimInput{
		InnerDiameterMM: 150,
		OuterDiameterMM: 225,
		WidthMM:         80,
		LoadN:           5000,
		SpeedRPM:        15,
		TempCelsius:     40,
		Hours:           8760,
		HardnessHV:      110,
		ArchardK:        1.5e-8,
		SurfaceRMS:      0.8e-6,
		ElasticModPa:    1.0e11,
		ViscosityPaS:    0.03,
		WearResistance:  0.85,
		IsRolling:       false,
	}

	out := wc.SimulateWear(in)

	if out.TotalWearUm <= 0 {
		t.Errorf("Bronze totalWearUm should be positive, got %f", out.TotalWearUm)
	}
	if out.WearRateUmPerHour <= 0 {
		t.Errorf("Bronze wearRateUmPerHour should be positive, got %f", out.WearRateUmPerHour)
	}
	if out.LifeHours <= 0 {
		t.Errorf("Bronze lifeHours should be positive, got %f", out.LifeHours)
	}
	if out.ContactPressureMPa <= 0 {
		t.Errorf("ContactPressureMPa should be positive, got %f", out.ContactPressureMPa)
	}
}

func TestSimulateWear_RollingVsSliding(t *testing.T) {
	wc := NewWearCalculator(2)
	defer wc.Close()

	baseInput := SimInput{
		InnerDiameterMM: 150,
		OuterDiameterMM: 225,
		WidthMM:         80,
		LoadN:           5000,
		SpeedRPM:        15,
		TempCelsius:     40,
		Hours:           8760,
		HardnessHV:      700,
		ArchardK:        5e-10,
		SurfaceRMS:      0.05e-6,
		ElasticModPa:    2.1e11,
		ViscosityPaS:    0.03,
		WearResistance:  5.0,
	}

	sliding := baseInput
	sliding.IsRolling = false
	rolling := baseInput
	rolling.IsRolling = true

	outSliding := wc.SimulateWear(sliding)
	outRolling := wc.SimulateWear(rolling)

	if outRolling.TotalWearUm >= outSliding.TotalWearUm {
		t.Errorf("Rolling wear (%f) should be less than sliding wear (%f)",
			outRolling.TotalWearUm, outSliding.TotalWearUm)
	}
	if outRolling.LifeHours <= outSliding.LifeHours {
		t.Errorf("Rolling life (%f) should exceed sliding life (%f)",
			outRolling.LifeHours, outSliding.LifeHours)
	}
}

func TestSimulateWear_ZeroLoad(t *testing.T) {
	wc := NewWearCalculator(2)
	defer wc.Close()

	in := SimInput{
		InnerDiameterMM: 150,
		OuterDiameterMM: 225,
		WidthMM:         80,
		LoadN:           0,
		SpeedRPM:        15,
		TempCelsius:     40,
		Hours:           8760,
		HardnessHV:      110,
		ArchardK:        1.5e-8,
		SurfaceRMS:      0.8e-6,
		ElasticModPa:    1.0e11,
		ViscosityPaS:    0.03,
		WearResistance:  0.85,
	}

	out := wc.SimulateWear(in)

	if out.TotalWearUm != 0 {
		t.Errorf("Zero load should produce zero wear, got %f", out.TotalWearUm)
	}
}

func TestCalculateViscosityWalther(t *testing.T) {
	wc := NewWearCalculator(1)
	defer wc.Close()

	visc40 := wc.CalculateViscosityWalther(40.0, 9.225, 3.540, 30.0, 0.03)
	if visc40 <= 0 {
		t.Errorf("Viscosity at 40C should be positive, got %f", visc40)
	}

	visc100 := wc.CalculateViscosityWalther(100.0, 9.225, 3.540, 30.0, 0.03)
	if visc100 <= 0 {
		t.Errorf("Viscosity at 100C should be positive, got %f", visc100)
	}

	if visc100 >= visc40 {
		t.Errorf("Viscosity should decrease with temperature: 40C=%f, 100C=%f", visc40, visc100)
	}
}

func TestCalculateEHLFilm(t *testing.T) {
	wc := NewWearCalculator(1)
	defer wc.Close()

	hMin := wc.CalculateEHLFilm(0.1, 15.0, 0.03, 5000.0, 2.1e11, 2.2e-8)
	if hMin <= 0 {
		t.Errorf("EHL film thickness should be positive, got %e", hMin)
	}

	hMinZeroRadius := wc.CalculateEHLFilm(0, 15.0, 0.03, 5000.0, 2.1e11, 2.2e-8)
	if hMinZeroRadius <= 0 {
		t.Errorf("EHL should handle zero radius gracefully")
	}
}

func TestWearCoefficientForLambda(t *testing.T) {
	wc := NewWearCalculator(1)
	defer wc.Close()

	kBoundary := wc.WearCoefficientForLambda(0.5)
	kMixed := wc.WearCoefficientForLambda(2.0)
	kFullFilm := wc.WearCoefficientForLambda(4.0)

	if kBoundary <= kMixed {
		t.Errorf("Boundary K (%f) should exceed mixed K (%f)", kBoundary, kMixed)
	}
	if kMixed <= kFullFilm {
		t.Errorf("Mixed K (%f) should exceed full-film K (%f)", kMixed, kFullFilm)
	}
}

func TestSimulateWearAsync(t *testing.T) {
	wc := NewWearCalculator(2)
	defer wc.Close()

	in := SimInput{
		InnerDiameterMM: 150,
		OuterDiameterMM: 225,
		WidthMM:         80,
		LoadN:           5000,
		SpeedRPM:        15,
		TempCelsius:     40,
		Hours:           8760,
		HardnessHV:      110,
		ArchardK:        1.5e-8,
		SurfaceRMS:      0.8e-6,
		ElasticModPa:    1.0e11,
		ViscosityPaS:    0.03,
		WearResistance:  0.85,
	}

	ch := wc.SimulateWearAsync(in)
	result := <-ch
	if result == nil {
		t.Fatal("Async result should not be nil")
	}
	if result.TotalWearUm <= 0 {
		t.Errorf("Async wear should be positive, got %f", result.TotalWearUm)
	}
}

func TestBatchSimulate(t *testing.T) {
	wc := NewWearCalculator(2)
	defer wc.Close()

	inputs := []SimInput{
		{InnerDiameterMM: 150, OuterDiameterMM: 225, WidthMM: 80, LoadN: 5000, SpeedRPM: 15, TempCelsius: 40, Hours: 8760, HardnessHV: 110, ArchardK: 1.5e-8, SurfaceRMS: 0.8e-6, ElasticModPa: 1e11, ViscosityPaS: 0.03, WearResistance: 0.85},
		{InnerDiameterMM: 150, OuterDiameterMM: 225, WidthMM: 80, LoadN: 5000, SpeedRPM: 15, TempCelsius: 40, Hours: 8760, HardnessHV: 700, ArchardK: 5e-10, SurfaceRMS: 0.05e-6, ElasticModPa: 2.1e11, ViscosityPaS: 0.03, WearResistance: 5.0, IsRolling: true},
	}

	results := BatchSimulate(wc, inputs)
	if len(results) != 2 {
		t.Fatalf("Expected 2 results, got %d", len(results))
	}

	if results[0].TotalWearUm <= 0 || results[1].TotalWearUm <= 0 {
		t.Errorf("Both results should have positive wear")
	}

	if results[0].TotalWearUm <= results[1].TotalWearUm {
		t.Errorf("Bronze wear (%f) should exceed rolling bearing wear (%f)",
			results[0].TotalWearUm, results[1].TotalWearUm)
	}
}

func TestWaltherExtremeTemp(t *testing.T) {
	wc := NewWearCalculator(1)
	defer wc.Close()

	viscNeg := wc.CalculateViscosityWalther(-274, 9.225, 3.540, 30.0, 0.03)
	if viscNeg != 0.03 {
		t.Errorf("Below absolute zero should return fallback, got %f", viscNeg)
	}

	viscHigh := wc.CalculateViscosityWalther(200, 9.225, 3.540, 30.0, 0.03)
	if viscHigh <= 0 {
		t.Errorf("High temp viscosity should still be positive, got %f", viscHigh)
	}
}

func TestSimOutputRegime(t *testing.T) {
	wc := NewWearCalculator(1)
	defer wc.Close()

	in := SimInput{
		InnerDiameterMM: 150,
		OuterDiameterMM: 225,
		WidthMM:         80,
		LoadN:           5000,
		SpeedRPM:        15,
		TempCelsius:     40,
		Hours:           8760,
		HardnessHV:      110,
		ArchardK:        1.5e-8,
		SurfaceRMS:      0.8e-6,
		ElasticModPa:    1.0e11,
		ViscosityPaS:    0.03,
		WearResistance:  0.85,
	}

	out := wc.SimulateWear(in)
	validRegimes := map[string]bool{"boundary": true, "mixed": true, "full_film": true}
	if !validRegimes[out.Regime] {
		t.Errorf("Invalid regime: %s", out.Regime)
	}
}

func TestWearLimitByDiameter(t *testing.T) {
	wc := NewWearCalculator(1)
	defer wc.Close()

	smallInput := SimInput{
		InnerDiameterMM: 50, OuterDiameterMM: 80, WidthMM: 30, LoadN: 5000,
		SpeedRPM: 15, TempCelsius: 40, Hours: 8760, HardnessHV: 110, ArchardK: 1.5e-8,
		SurfaceRMS: 0.8e-6, ElasticModPa: 1e11, ViscosityPaS: 0.03, WearResistance: 0.85,
	}
	largeInput := smallInput
	largeInput.InnerDiameterMM = 250
	largeInput.OuterDiameterMM = 375

	smallOut := wc.SimulateWear(smallInput)
	largeOut := wc.SimulateWear(largeInput)

	if largeOut.WearLimitUm <= smallOut.WearLimitUm {
		t.Errorf("Larger bearing should have higher wear limit: large=%f, small=%f",
			largeOut.WearLimitUm, smallOut.WearLimitUm)
	}
}

func TestBatchSimulateConsistency(t *testing.T) {
	wc := NewWearCalculator(2)
	defer wc.Close()

	in := SimInput{
		InnerDiameterMM: 150, OuterDiameterMM: 225, WidthMM: 80, LoadN: 5000,
		SpeedRPM: 15, TempCelsius: 40, Hours: 8760, HardnessHV: 110, ArchardK: 1.5e-8,
		SurfaceRMS: 0.8e-6, ElasticModPa: 1e11, ViscosityPaS: 0.03, WearResistance: 0.85,
	}

	syncOut := wc.SimulateWear(in)
	asyncCh := wc.SimulateWearAsync(in)
	asyncOut := <-asyncCh

	diff := math.Abs(syncOut.TotalWearUm - asyncOut.TotalWearUm)
	if diff > 1e-10 {
		t.Errorf("Sync and async results should match: sync=%f, async=%f, diff=%e",
			syncOut.TotalWearUm, asyncOut.TotalWearUm, diff)
	}
}
