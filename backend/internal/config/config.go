package config

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/viper"
)

type Config struct {
	Server        ServerConfig        `mapstructure:"server"`
	Database      DatabaseConfig      `mapstructure:"database"`
	MQTT          MQTTConfig          `mapstructure:"mqtt"`
	WearCalc      WearCalcConfig      `mapstructure:"wear_calculation"`
	LifePred      LifePredConfig      `mapstructure:"life_prediction"`
	Alert         AlertConfig         `mapstructure:"alert"`
	WearParams    WearParamsConfig
	Lubrication   LubricationConfig
	Materials     BearingMaterialsConfig
	Lubricants    LubricantsConfig
}

type ServerConfig struct {
	Port        int      `mapstructure:"port"`
	ModbusPort  int      `mapstructure:"modbus_port"`
	CORSOrigins []string `mapstructure:"cors_origins"`
}

type DatabaseConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	DBName   string `mapstructure:"dbname"`
	SSLMode  string `mapstructure:"sslmode"`
}

type MQTTConfig struct {
	Broker      string `mapstructure:"broker"`
	ClientID    string `mapstructure:"client_id"`
	Username    string `mapstructure:"username"`
	Password    string `mapstructure:"password"`
	TopicPrefix string `mapstructure:"topic_prefix"`
}

type WearCalcConfig struct {
	IntervalMinutes  int     `mapstructure:"interval_minutes"`
	ArchardK         float64 `mapstructure:"archard_k"`
	EHLReferenceTemp float64 `mapstructure:"ehl_reference_temp"`
}

type LifePredConfig struct {
	IntervalMinutes       int     `mapstructure:"interval_minutes"`
	WeibullDefaultShape   float64 `mapstructure:"weibull_default_shape"`
	WeibullDefaultScale   float64 `mapstructure:"weibull_default_scale"`
	MinSamplesForFit      int     `mapstructure:"min_samples_for_fit"`
}

type AlertConfig struct {
	WearWarningRatio   float64 `mapstructure:"wear_warning_ratio"`
	WearCriticalRatio  float64 `mapstructure:"wear_critical_ratio"`
	OilFilmMinimum     float64 `mapstructure:"oil_film_minimum"`
	CooldownMinutes    int     `mapstructure:"cooldown_minutes"`
}

type WearParamsConfig struct {
	ArchardKBase                 float64           `json:"archard_k_base"`
	EHLReferenceTempCelsius      float64           `json:"ehl_reference_temp_celsius"`
	PressureViscosityCoefficient float64           `json:"pressure_viscosity_coefficient"`
	ReducedElasticModulusPa      float64           `json:"reduced_elastic_modulus_pa"`
	SurfaceRoughnessRMSMeters    float64           `json:"surface_roughness_rms_meters"`
	TempCorrectionFactorPerDegree float64          `json:"temp_correction_factor_per_degree"`
	FullFilmLambdaThreshold      float64           `json:"full_film_lambda_threshold"`
	MixedLubricationLambdaThreshold float64        `json:"mixed_lubrication_lambda_threshold"`
	WearCoefficientFactors       map[string]float64 `json:"wear_coefficient_factors"`
	HardnessConversionFactor     float64           `json:"hardness_conversion_factor"`
	OilFilmGrid                  OilFilmGridConfig `json:"oil_film_grid"`
}

type OilFilmGridConfig struct {
	SizeX int `json:"size_x"`
	SizeY int `json:"size_y"`
}

type LubricationConfig struct {
	MixedLubrication      MixedLubricationConfig      `json:"mixed_lubrication"`
	EHLCorrection         EHLCorrectionConfig         `json:"ehl_correction"`
	OilFilmRuptureThresholdMicrom float64             `json:"oil_film_rupture_threshold_microm"`
	ViscosityIndex        int                           `json:"viscosity_index"`
	PressureCoefficientAlpha float64                    `json:"pressure_coefficient_alpha"`
}

type MixedLubricationConfig struct {
	SommerfeldThresholdLow         float64 `json:"sommerfeld_threshold_low"`
	SommerfeldThresholdHigh        float64 `json:"sommerfeld_threshold_high"`
	CorrectionFactorLowSommerfeld  float64 `json:"correction_factor_low_sommerfeld"`
	LoadSeverityExponent           float64 `json:"load_severity_exponent"`
	LoadSeverityReferenceN         float64 `json:"load_severity_reference_n"`
	AsperityContactExponent        float64 `json:"asperity_contact_exponent"`
	AsperityEffectFactor           float64 `json:"asperity_effect_factor"`
	BoundaryMaxContactRatio        float64 `json:"boundary_max_contact_ratio"`
}

type EHLCorrectionConfig struct {
	DowsonHigginsonCoefficient     float64 `json:"dowson_higginson_coefficient"`
	AlternativeFormulaCoefficient  float64 `json:"alternative_formula_coefficient"`
	AlternativeFormulaThresholdU   float64 `json:"alternative_formula_threshold_u"`
	TempViscosityExponent          float64 `json:"temp_viscosity_exponent"`
	LambdaMinClamp                 float64 `json:"lambda_min_clamp"`
	LambdaMaxClamp                 float64 `json:"lambda_max_clamp"`
}

type BearingMaterial struct {
	Code                    string   `json:"code"`
	NameCN                  string   `json:"name_cn"`
	Era                     string   `json:"era"`
	Category                string   `json:"category"`
	HardnessHVMin           float64  `json:"hardness_hv_min"`
	HardnessHVNominal       float64  `json:"hardness_hv_nominal"`
	HardnessHVMax           float64  `json:"hardness_hv_max"`
	DensityKgPerM3          float64  `json:"density_kg_per_m3"`
	ElasticModulusPa        float64  `json:"elastic_modulus_pa"`
	PoissonRatio            float64  `json:"poisson_ratio"`
	ThermalConductivity     float64  `json:"thermal_conductivity_w_per_mk"`
	SurfaceRoughnessRMS     float64  `json:"surface_roughness_rms_meters"`
	ArchardKBase            float64  `json:"archard_k_base"`
	WearResistanceFactor    float64  `json:"wear_resistance_factor"`
	CorrosionResistance     float64  `json:"corrosion_resistance"`
	TemperatureLimitCelsius float64  `json:"temperature_limit_celsius"`
	ManufacturingDifficulty float64  `json:"manufacturing_difficulty"`
	HistoricalNote          string   `json:"historical_note"`
	TypicalApplications     []string `json:"typical_applications"`
}

type BearingMaterialsConfig struct {
	Materials          []BearingMaterial  `json:"materials"`
	EraDefinitions     map[string]string  `json:"era_definitions"`
	CategoryDefinitions map[string]string `json:"category_definitions"`
	materialIndex      map[string]BearingMaterial
}

type Lubricant struct {
	Code                      string   `json:"code"`
	NameCN                    string   `json:"name_cn"`
	Category                  string   `json:"category"`
	Era                       string   `json:"era"`
	BaseOilType               string   `json:"base_oil_type"`
	Viscosity40C              float64  `json:"viscosity_40c_mm2_per_s"`
	Viscosity100C             float64  `json:"viscosity_100c_mm2_per_s"`
	ViscosityIndex            int      `json:"viscosity_index"`
	ViscosityPaSAt40C         float64  `json:"viscosity_pas_at_40c"`
	PressureViscosityCoeff    float64  `json:"pressure_viscosity_coefficient"`
	PourPointCelsius          float64  `json:"pour_point_celsius"`
	FlashPointCelsius         float64  `json:"flash_point_celsius"`
	OxidationStability        float64  `json:"oxidation_stability_index"`
	AntiWearPerformance       float64  `json:"anti_wear_performance"`
	LoadCarryingCapacity      float64  `json:"load_carrying_capacity"`
	CorrosionInhibition       float64  `json:"corrosion_inhibition"`
	FrictionCoefficientDry    float64  `json:"friction_coefficient_dry"`
	FrictionCoefficientLubed  float64  `json:"friction_coefficient_lubricated"`
	LubricationEffectiveness  float64  `json:"lubrication_effectiveness_factor"`
	EHLBoostFactor            float64  `json:"ehl_boost_factor"`
	WearReductionRatio        float64  `json:"wear_reduction_ratio"`
	DegradationRate           float64  `json:"degradation_rate_per_1000h"`
	MaxLubricationLifeHours   float64  `json:"max_lubrication_life_hours"`
	HistoricalNote            string   `json:"historical_note"`
	TypicalApplications       []string `json:"typical_applications"`
	TypicalSourceRegions      []string `json:"typical_source_regions"`
}

type LubricantsConfig struct {
	Lubricants                     []Lubricant       `json:"lubricants"`
	CategoryDefinitions            map[string]string `json:"category_definitions"`
	RecommendedLubricationFreq     map[string]float64 `json:"recommended_lubrication_frequency_hours"`
	lubricantIndex                 map[string]Lubricant
}

var AppConfig *Config

func Load(path string) error {
	viper.SetConfigFile(path)
	viper.SetConfigType("yaml")

	if err := viper.ReadInConfig(); err != nil {
		return fmt.Errorf("读取配置文件失败: %w", err)
	}

	cfg := &Config{}
	if err := viper.Unmarshal(cfg); err != nil {
		return fmt.Errorf("解析配置失败: %w", err)
	}

	if err := loadWearParams("config/wear_params.json", &cfg.WearParams); err != nil {
		return fmt.Errorf("加载磨损参数失败: %w", err)
	}

	if err := loadLubricationParams("config/lubrication_params.json", &cfg.Lubrication); err != nil {
		return fmt.Errorf("加载润滑参数失败: %w", err)
	}

	if err := loadMaterialsConfig("config/bearing_materials.json", &cfg.Materials); err != nil {
		return fmt.Errorf("加载轴承材料配置失败: %w", err)
	}

	if err := loadLubricantsConfig("config/lubricants.json", &cfg.Lubricants); err != nil {
		return fmt.Errorf("加载润滑剂配置失败: %w", err)
	}

	AppConfig = cfg
	return nil
}

func loadWearParams(path string, params *WearParamsConfig) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("读取磨损参数文件失败: %w", err)
	}
	if err := json.Unmarshal(data, params); err != nil {
		return fmt.Errorf("解析磨损参数失败: %w", err)
	}
	return nil
}

func loadLubricationParams(path string, params *LubricationConfig) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("读取润滑参数文件失败: %w", err)
	}
	if err := json.Unmarshal(data, params); err != nil {
		return fmt.Errorf("解析润滑参数失败: %w", err)
	}
	return nil
}

func loadMaterialsConfig(path string, cfg *BearingMaterialsConfig) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("读取轴承材料文件失败: %w", err)
	}
	if err := json.Unmarshal(data, cfg); err != nil {
		return fmt.Errorf("解析轴承材料配置失败: %w", err)
	}
	cfg.BuildIndex()
	return nil
}

func loadLubricantsConfig(path string, cfg *LubricantsConfig) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("读取润滑剂配置文件失败: %w", err)
	}
	if err := json.Unmarshal(data, cfg); err != nil {
		return fmt.Errorf("解析润滑剂配置失败: %w", err)
	}
	cfg.BuildIndex()
	return nil
}

func (bm *BearingMaterialsConfig) BuildIndex() {
	bm.materialIndex = make(map[string]BearingMaterial, len(bm.Materials))
	for _, m := range bm.Materials {
		bm.materialIndex[m.Code] = m
	}
}

func (bm *BearingMaterialsConfig) GetMaterial(code string) (BearingMaterial, bool) {
	m, ok := bm.materialIndex[code]
	return m, ok
}

func (bm *BearingMaterialsConfig) ListByEra(era string) []BearingMaterial {
	result := make([]BearingMaterial, 0)
	for _, m := range bm.Materials {
		if m.Era == era {
			result = append(result, m)
		}
	}
	return result
}

func (bm *BearingMaterialsConfig) ListByCategory(category string) []BearingMaterial {
	result := make([]BearingMaterial, 0)
	for _, m := range bm.Materials {
		if m.Category == category {
			result = append(result, m)
		}
	}
	return result
}

func (lc *LubricantsConfig) BuildIndex() {
	lc.lubricantIndex = make(map[string]Lubricant, len(lc.Lubricants))
	for _, l := range lc.Lubricants {
		lc.lubricantIndex[l.Code] = l
	}
}

func (lc *LubricantsConfig) GetLubricant(code string) (Lubricant, bool) {
	l, ok := lc.lubricantIndex[code]
	return l, ok
}

func (lc *LubricantsConfig) ListByCategory(category string) []Lubricant {
	result := make([]Lubricant, 0)
	for _, l := range lc.Lubricants {
		if l.Category == category {
			result = append(result, l)
		}
	}
	return result
}

func (lc *LubricantsConfig) ListByEra(era string) []Lubricant {
	result := make([]Lubricant, 0)
	for _, l := range lc.Lubricants {
		if l.Era == era || l.Era == "ancient" || l.Era == "ancient_rare" {
			if era == "ancient" || (era == "modern" && l.Era == "modern") || (era == "ancient" && (l.Era == "ancient" || l.Era == "ancient_rare")) {
				if era == "modern" && l.Era != "modern" {
					continue
				}
				result = append(result, l)
			}
		}
	}
	return result
}

func (d DatabaseConfig) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		d.Host, d.Port, d.User, d.Password, d.DBName, d.SSLMode,
	)
}
