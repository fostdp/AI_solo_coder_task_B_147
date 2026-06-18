package wearcalc

import (
	"math"
	"sync"

	"noria-bearing-system/internal/config"
)

type SimInput struct {
	InnerDiameterMM float64
	OuterDiameterMM float64
	WidthMM         float64
	LoadN           float64
	SpeedRPM        float64
	TempCelsius     float64
	Hours           float64
	HardnessHV      float64
	ArchardK        float64
	SurfaceRMS      float64
	ElasticModPa    float64
	ViscosityPaS    float64
	Viscosity40C    float64
	WaltherA        float64
	WaltherB        float64
	PressureCoeff   float64
	EHLBoost        float64
	WearReduction   float64
	WearResistance  float64
	IsRolling       bool
}

type SimOutput struct {
	TotalWearUm        float64
	WearRateUmPerHour  float64
	EHLMeanLambda      float64
	ContactPressureMPa float64
	ArchardVolumeM3    float64
	LifeHours          float64
	LifeYears          float64
	WearLimitUm        float64
	Regime             string
}

const WearLimitDefaultUm = 500.0

type WearCalculator struct {
	wearParams  *config.WearParamsConfig
	lubrication *config.LubricationConfig
	workerCount int
	taskQueue   chan *wearCalcTask
	wg          sync.WaitGroup
	mu          sync.RWMutex
}

type wearCalcTask struct {
	input    SimInput
	resultCh chan *SimOutput
}

func NewWearCalculator(workerCount int) *WearCalculator {
	wc := &WearCalculator{
		wearParams:  &config.AppConfig.WearParams,
		lubrication: &config.AppConfig.Lubrication,
		workerCount: workerCount,
		taskQueue:   make(chan *wearCalcTask, workerCount*8),
	}
	if wc.workerCount <= 0 {
		wc.workerCount = 2
	}
	wc.startWorkers()
	return wc
}

func (wc *WearCalculator) startWorkers() {
	for i := 0; i < wc.workerCount; i++ {
		wc.wg.Add(1)
		go func() {
			defer wc.wg.Done()
			for task := range wc.taskQueue {
				result := wc.SimulateWear(task.input)
				task.resultCh <- &result
				close(task.resultCh)
			}
		}()
	}
}

func (wc *WearCalculator) Close() {
	close(wc.taskQueue)
	wc.wg.Wait()
}

func (wc *WearCalculator) SimulateWearAsync(in SimInput) <-chan *SimOutput {
	resultCh := make(chan *SimOutput, 1)
	wc.taskQueue <- &wearCalcTask{input: in, resultCh: resultCh}
	return resultCh
}

func (wc *WearCalculator) SimulateWear(in SimInput) SimOutput {
	wc.mu.RLock()
	defer wc.mu.RUnlock()
	return wc.simulateWearInternal(in)
}

func (wc *WearCalculator) simulateWearInternal(in SimInput) SimOutput {
	out := SimOutput{}

	innerRadius := in.InnerDiameterMM / 2.0 / 1000.0
	outerRadius := in.OuterDiameterMM / 2.0 / 1000.0
	effectiveRadius := (innerRadius + outerRadius) / 2.0
	widthMeters := in.WidthMM / 1000.0

	contactArea := math.Pi * (outerRadius*outerRadius - innerRadius*innerRadius)
	if contactArea <= 0 {
		contactArea = effectiveRadius * 2 * math.Pi * widthMeters
	}

	contactPressurePa := in.LoadN / contactArea
	out.ContactPressureMPa = contactPressurePa / 1e6

	rpmToRadPerSec := 2.0 * math.Pi / 60.0
	angularVelocity := in.SpeedRPM * rpmToRadPerSec
	slidingVelocity := effectiveRadius * angularVelocity

	if in.IsRolling {
		slidingVelocity *= 0.02
	}

	periodSeconds := in.Hours * 3600.0
	slidingDistance := slidingVelocity * periodSeconds

	hardnessPa := in.HardnessHV * wc.wearParams.HardnessConversionFactor

	viscosity := in.ViscosityPaS
	if viscosity <= 0 {
		viscosity = 0.03
	}

	if in.WaltherA > 0 && in.WaltherB > 0 && in.Viscosity40C > 0 {
		viscosity = wc.CalculateViscosityWalther(
			in.TempCelsius,
			in.WaltherA,
			in.WaltherB,
			in.Viscosity40C,
			in.ViscosityPaS,
		)
	} else {
		refTemp := wc.wearParams.EHLReferenceTempCelsius
		tempCorrection := math.Exp(-0.05 * (in.TempCelsius - refTemp) * 0.693 / 10.0)
		viscosity = in.ViscosityPaS * tempCorrection
	}
	effectiveViscosity := viscosity

	filmThickness := wc.CalculateEHLFilm(
		effectiveRadius,
		in.SpeedRPM,
		effectiveViscosity,
		in.LoadN,
		in.ElasticModPa,
		in.PressureCoeff,
	)

	filmThickness *= in.EHLBoost

	out.EHLMeanLambda = filmThickness / in.SurfaceRMS
	if out.EHLMeanLambda < wc.lubrication.EHLCorrection.LambdaMinClamp {
		out.EHLMeanLambda = wc.lubrication.EHLCorrection.LambdaMinClamp
	}
	if out.EHLMeanLambda > wc.lubrication.EHLCorrection.LambdaMaxClamp {
		out.EHLMeanLambda = wc.lubrication.EHLCorrection.LambdaMaxClamp
	}

	switch {
	case out.EHLMeanLambda >= wc.wearParams.FullFilmLambdaThreshold:
		out.Regime = "full_film"
	case out.EHLMeanLambda >= wc.wearParams.MixedLubricationLambdaThreshold:
		out.Regime = "mixed"
	default:
		out.Regime = "boundary"
	}

	wearCoefficient := wc.WearCoefficientForLambda(out.EHLMeanLambda)
	wearCoefficient *= (1.0 - in.WearReduction*0.8)
	wearCoefficient = wearCoefficient / in.WearResistance

	tempFactor := 1.0
	refTemp := wc.wearParams.EHLReferenceTempCelsius
	if in.TempCelsius > refTemp {
		tempFactor = 1.0 + wc.wearParams.TempCorrectionFactorPerDegree*(in.TempCelsius-refTemp)
	}

	archardVolume := in.ArchardK * wearCoefficient * in.LoadN * slidingDistance / hardnessPa * tempFactor
	if in.IsRolling {
		archardVolume *= 0.05
	}

	wearDepthMeters := archardVolume / contactArea
	out.TotalWearUm = wearDepthMeters * 1e6
	out.ArchardVolumeM3 = archardVolume

	if in.Hours > 0 {
		out.WearRateUmPerHour = out.TotalWearUm / in.Hours
	}

	wearLimit := WearLimitDefaultUm
	if in.InnerDiameterMM >= 200 {
		wearLimit = 1000.0
	} else if in.InnerDiameterMM >= 100 {
		wearLimit = 750.0
	}

	if in.IsRolling {
		wearLimit *= 0.3
	}

	out.WearLimitUm = wearLimit

	if out.WearRateUmPerHour > 0 {
		out.LifeHours = wearLimit / out.WearRateUmPerHour * 0.9
	} else {
		out.LifeHours = in.Hours * 1000
	}

	if out.LifeHours > 200000 {
		out.LifeHours = 200000
	}

	out.LifeYears = out.LifeHours / 8760.0

	return out
}

func (wc *WearCalculator) CalculateViscosityWalther(tempCelsius, waltherA, waltherB, viscosity40C, viscosityPaS40C float64) float64 {
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

func (wc *WearCalculator) CalculateEHLFilm(
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

	if W <= 1e-10 {
		W = 1e-10
	}
	if U <= 1e-15 {
		U = 1e-15
	}

	DHcoeff := wc.lubrication.EHLCorrection.DowsonHigginsonCoefficient

	hMin := DHcoeff * effRadius *
		math.Pow(U, 0.68) *
		math.Pow(G, 0.49) *
		math.Pow(W, -0.073)

	if hMin <= 0 {
		return 1e-7
	}

	return hMin
}

func (wc *WearCalculator) WearCoefficientForLambda(lambda float64) float64 {
	factors := wc.wearParams.WearCoefficientFactors
	boundary := factors["boundary"]
	mixed := factors["mixed"]
	fullFilm := factors["full_film"]
	if boundary <= 0 {
		boundary = 100.0
	}
	if mixed <= 0 {
		mixed = 10.0
	}
	if fullFilm <= 0 {
		fullFilm = 1.0
	}

	fullThreshold := wc.wearParams.FullFilmLambdaThreshold
	mixedThreshold := wc.wearParams.MixedLubricationLambdaThreshold

	if lambda >= fullThreshold {
		return fullFilm
	}
	if lambda >= mixedThreshold {
		ratio := (lambda - mixedThreshold) / (fullThreshold - mixedThreshold)
		return mixed*(1.0-ratio) + fullFilm*ratio
	}
	ratio := lambda / mixedThreshold
	return boundary*(1.0-ratio) + mixed*ratio
}

func BatchSimulate(wc *WearCalculator, inputs []SimInput) []SimOutput {
	results := make([]SimOutput, len(inputs))
	chans := make([]<-chan *SimOutput, len(inputs))

	for i, in := range inputs {
		chans[i] = wc.SimulateWearAsync(in)
	}

	for i, ch := range chans {
		res := <-ch
		if res != nil {
			results[i] = *res
		}
	}

	return results
}
