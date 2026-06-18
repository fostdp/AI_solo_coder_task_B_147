package models

import (
	"time"
)

type NoriaWheel struct {
	ID              int       `json:"id" db:"id"`
	Name            string    `json:"name" db:"name"`
	Location        string    `json:"location" db:"location"`
	Diameter        float64   `json:"diameter" db:"diameter"`
	Buckets         int       `json:"buckets" db:"buckets"`
	InstallationDate *string  `json:"installation_date,omitempty" db:"installation_date"`
	Description     string    `json:"description" db:"description"`
	CreatedAt       time.Time `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time `json:"updated_at" db:"updated_at"`
}

type Bearing struct {
	ID                 int       `json:"id" db:"id"`
	NoriaWheelID       int       `json:"noria_wheel_id" db:"noria_wheel_id"`
	BearingCode        string    `json:"bearing_code" db:"bearing_code"`
	Position           string    `json:"position" db:"position"`
	BearingType        string    `json:"bearing_type" db:"bearing_type"`
	InnerDiameter      float64   `json:"inner_diameter" db:"inner_diameter"`
	OuterDiameter      float64   `json:"outer_diameter" db:"outer_diameter"`
	Width              float64   `json:"width" db:"width"`
	Material           string    `json:"material" db:"material"`
	HardnessHV         float64   `json:"hardness_hv" db:"hardness_hv"`
	RatedLifeHours     float64   `json:"rated_life_hours" db:"rated_life_hours"`
	WearLimitMicrom    float64   `json:"wear_limit_microm" db:"wear_limit_microm"`
	InitialWearMicrom  float64   `json:"initial_wear_microm" db:"initial_wear_microm"`
	LubricationType    string    `json:"lubrication_type" db:"lubrication_type"`
	OilViscosityPaS    float64   `json:"oil_viscosity_pas" db:"oil_viscosity_pas"`
	InstalledAt        time.Time `json:"installed_at" db:"installed_at"`
	Status             string    `json:"status" db:"status"`
	CreatedAt          time.Time `json:"created_at" db:"created_at"`
}

type SensorData struct {
	Time              time.Time `json:"time" db:"time"`
	BearingID         int       `json:"bearing_id" db:"bearing_id"`
	Temperature       float64   `json:"temperature" db:"temperature"`
	RadialLoad        float64   `json:"radial_load" db:"radial_load"`
	RotationalSpeed   float64   `json:"rotational_speed" db:"rotational_speed"`
	OilFilmThickness  float64   `json:"oil_film_thickness" db:"oil_film_thickness"`
	AmbientTemp       *float64  `json:"ambient_temp,omitempty" db:"ambient_temp"`
	Humidity          *float64  `json:"humidity,omitempty" db:"humidity"`
	Source            string    `json:"source" db:"source"`
}

type WearResult struct {
	ID                    int64      `json:"id" db:"id"`
	BearingID             int        `json:"bearing_id" db:"bearing_id"`
	CalculatedAt          time.Time  `json:"calculated_at" db:"calculated_at"`
	PeriodStart           time.Time  `json:"period_start" db:"period_start"`
	PeriodEnd             time.Time  `json:"period_end" db:"period_end"`
	WearDepthMicrom       float64    `json:"wear_depth_microm" db:"wear_depth_microm"`
	WearRateMicromPerHour *float64   `json:"wear_rate_microm_per_hour,omitempty" db:"wear_rate_microm_per_hour"`
	TotalWearMicrom       float64    `json:"total_wear_microm" db:"total_wear_microm"`
	ArchardWearVolume     *float64   `json:"archard_wear_volume,omitempty" db:"archard_wear_volume"`
	EHLFilmParameter      *float64   `json:"ehl_film_parameter,omitempty" db:"ehl_film_parameter"`
	SlidingDistance       *float64   `json:"sliding_distance,omitempty" db:"sliding_distance"`
	WearCoefficient       *float64   `json:"wear_coefficient,omitempty" db:"wear_coefficient"`
	ContactPressure       *float64   `json:"contact_pressure,omitempty" db:"contact_pressure"`
	CalculationNote       *string    `json:"calculation_note,omitempty" db:"calculation_note"`
}

type LifePrediction struct {
	ID                      int64     `json:"id" db:"id"`
	BearingID               int       `json:"bearing_id" db:"bearing_id"`
	PredictedAt             time.Time `json:"predicted_at" db:"predicted_at"`
	WeibullShape            float64   `json:"weibull_shape" db:"weibull_shape"`
	WeibullScale            float64   `json:"weibull_scale" db:"weibull_scale"`
	RunningHours            float64   `json:"running_hours" db:"running_hours"`
	PredictedRULHours       float64   `json:"predicted_rul_hours" db:"predicted_rul_hours"`
	Reliability             *float64  `json:"reliability,omitempty" db:"reliability"`
	FailureProbability      *float64  `json:"failure_probability,omitempty" db:"failure_probability"`
	ConfidenceIntervalLow   *float64  `json:"confidence_interval_low,omitempty" db:"confidence_interval_low"`
	ConfidenceIntervalHigh  *float64  `json:"confidence_interval_high,omitempty" db:"confidence_interval_high"`
	WearRateTrend           *float64  `json:"wear_rate_trend,omitempty" db:"wear_rate_trend"`
	FatigueDamage           *float64  `json:"fatigue_damage,omitempty" db:"fatigue_damage"`
	PredictionMethod        string    `json:"prediction_method" db:"prediction_method"`
}

type AlertEvent struct {
	ID             int64      `json:"id" db:"id"`
	BearingID      int        `json:"bearing_id" db:"bearing_id"`
	AlertTime      time.Time  `json:"alert_time" db:"alert_time"`
	AlertType      string     `json:"alert_type" db:"alert_type"`
	AlertLevel     string     `json:"alert_level" db:"alert_level"`
	AlertMessage   string     `json:"alert_message" db:"alert_message"`
	ThresholdValue *float64   `json:"threshold_value,omitempty" db:"threshold_value"`
	ActualValue    *float64   `json:"actual_value,omitempty" db:"actual_value"`
	MQTTTopic      *string    `json:"mqtt_topic,omitempty" db:"mqtt_topic"`
	Acknowledged   bool       `json:"acknowledged" db:"acknowledged"`
	AcknowledgedAt *time.Time `json:"acknowledged_at,omitempty" db:"acknowledged_at"`
	Resolved       bool       `json:"resolved" db:"resolved"`
	ResolvedAt     *time.Time `json:"resolved_at,omitempty" db:"resolved_at"`
}

type BearingLatestStatus struct {
	BearingID             int        `json:"bearing_id" db:"bearing_id"`
	BearingCode           string     `json:"bearing_code" db:"bearing_code"`
	Position              string     `json:"position" db:"position"`
	BearingType           string     `json:"bearing_type" db:"bearing_type"`
	NoriaWheelID          int        `json:"noria_wheel_id" db:"noria_wheel_id"`
	NoriaName             string     `json:"noria_name" db:"noria_name"`
	LastDataTime          *time.Time `json:"last_data_time,omitempty" db:"last_data_time"`
	Temperature           *float64   `json:"temperature,omitempty" db:"temperature"`
	RadialLoad            *float64   `json:"radial_load,omitempty" db:"radial_load"`
	RotationalSpeed       *float64   `json:"rotational_speed,omitempty" db:"rotational_speed"`
	OilFilmThickness      *float64   `json:"oil_film_thickness,omitempty" db:"oil_film_thickness"`
	TotalWearMicrom       *float64   `json:"total_wear_microm,omitempty" db:"total_wear_microm"`
	WearRateMicromPerHour *float64   `json:"wear_rate_microm_per_hour,omitempty" db:"wear_rate_microm_per_hour"`
	PredictedRULHours     *float64   `json:"predicted_rul_hours,omitempty" db:"predicted_rul_hours"`
	Reliability           *float64   `json:"reliability,omitempty" db:"reliability"`
	HealthStatus          string     `json:"health_status" db:"health_status"`
}

type OilFilmMap struct {
	ID          int64                  `json:"id" db:"id"`
	BearingID   int                    `json:"bearing_id" db:"bearing_id"`
	CapturedAt  time.Time              `json:"captured_at" db:"captured_at"`
	GridSizeX   int                    `json:"grid_size_x" db:"grid_size_x"`
	GridSizeY   int                    `json:"grid_size_y" db:"grid_size_y"`
	FilmData    map[string]interface{} `json:"film_data" db:"film_data"`
}

type MaintenanceRecord struct {
	ID              int64      `json:"id" db:"id"`
	BearingID       int        `json:"bearing_id" db:"bearing_id"`
	PerformedAt     time.Time  `json:"performed_at" db:"performed_at"`
	MaintenanceType string     `json:"maintenance_type" db:"maintenance_type"`
	Action          string     `json:"action" db:"action"`
	OldMaterialCode *string    `json:"old_material_code,omitempty" db:"old_material_code"`
	NewMaterialCode *string    `json:"new_material_code,omitempty" db:"new_material_code"`
	LubricantCode   *string    `json:"lubricant_code,omitempty" db:"lubricant_code"`
	LubricantAmount *float64   `json:"lubricant_amount_ml,omitempty" db:"lubricant_amount_ml"`
	WearBeforeUm    *float64   `json:"wear_before_um,omitempty" db:"wear_before_um"`
	WearAfterUm     *float64   `json:"wear_after_um,omitempty" db:"wear_after_um"`
	OperatorName    *string    `json:"operator_name,omitempty" db:"operator_name"`
	Notes           *string    `json:"notes,omitempty" db:"notes"`
	UserSessionID   *string    `json:"user_session_id,omitempty" db:"user_session_id"`
}

type MaterialComparisonResult struct {
	BaseBearingID   int                     `json:"base_bearing_id"`
	ReferenceLoad   float64                 `json:"reference_load_n"`
	ReferenceSpeed  float64                 `json:"reference_speed_rpm"`
	ReferenceTemp   float64                 `json:"reference_temp_celsius"`
	SimulationHours float64                 `json:"simulation_hours"`
	Items           []MaterialComparisonItem `json:"items"`
	GeneratedAt     time.Time               `json:"generated_at"`
}

type MaterialComparisonItem struct {
	MaterialCode          string   `json:"material_code"`
	MaterialName          string   `json:"material_name"`
	Era                   string   `json:"era"`
	Category              string   `json:"category"`
	HardnessHV            float64  `json:"hardness_hv"`
	TotalWearMicrom       float64  `json:"total_wear_microm"`
	WearRateUmPerHour     float64  `json:"wear_rate_um_per_hour"`
	PredictedLifeHours    float64  `json:"predicted_life_hours"`
	PredictedLifeYears    float64  `json:"predicted_life_years"`
	EHLMeanLambda         float64  `json:"ehl_mean_lambda"`
	MaxContactPressureMPa float64  `json:"max_contact_pressure_mpa"`
	LifeRatioVsBest       float64  `json:"life_ratio_vs_best"`
	Rank                  int      `json:"rank"`
	HistoricalNote        string   `json:"historical_note"`
	TypicalApplications   []string `json:"typical_applications"`
}

type LubricantComparisonResult struct {
	BaseBearingID   int                        `json:"base_bearing_id"`
	BaseMaterial    string                     `json:"base_material_code"`
	ReferenceLoad   float64                    `json:"reference_load_n"`
	ReferenceSpeed  float64                    `json:"reference_speed_rpm"`
	ReferenceTemp   float64                    `json:"reference_temp_celsius"`
	SimulationHours float64                    `json:"simulation_hours"`
	Items           []LubricantComparisonItem  `json:"items"`
	GeneratedAt     time.Time                  `json:"generated_at"`
}

type LubricantComparisonItem struct {
	LubricantCode         string   `json:"lubricant_code"`
	LubricantName         string   `json:"lubricant_name"`
	Category              string   `json:"category"`
	Era                   string   `json:"era"`
	ViscosityAt40C        float64  `json:"viscosity_40c"`
	ViscosityIndex        int      `json:"viscosity_index"`
	TotalWearMicrom       float64  `json:"total_wear_microm"`
	WearRateUmPerHour     float64  `json:"wear_rate_um_per_hour"`
	PredictedLifeHours    float64  `json:"predicted_life_hours"`
	PredictedLifeYears    float64  `json:"predicted_life_years"`
	EHLMeanLambda         float64  `json:"ehl_mean_lambda"`
	LubricationRegime     string   `json:"lubrication_regime"`
	WearReductionVsDry    float64  `json:"wear_reduction_vs_dry_pct"`
	LifeExtensionVsDry    float64  `json:"life_extension_vs_dry_pct"`
	LifeRatioVsBest       float64  `json:"life_ratio_vs_best"`
	Rank                  int      `json:"rank"`
	RecommendedFreqHours  float64  `json:"recommended_frequency_hours"`
	HistoricalNote        string   `json:"historical_note"`
}

type CrossEraComparisonResult struct {
	ReferenceLoad        float64                      `json:"reference_load_n"`
	ReferenceSpeed       float64                      `json:"reference_speed_rpm"`
	ReferenceTemp        float64                      `json:"reference_temp_celsius"`
	SimulationHours      float64                      `json:"simulation_hours"`
	BearingDiameter      float64                      `json:"bearing_diameter_mm"`
	AncientBest          *MaterialComparisonItem      `json:"ancient_best"`
	ModernBest           *MaterialComparisonItem      `json:"modern_best"`
	AllItems             []EraComparisonItem          `json:"all_items"`
	LifeImprovementX     float64                      `json:"life_improvement_x_times"`
	WearReductionPct     float64                      `json:"wear_reduction_percent"`
	InsightSummary       []string                     `json:"insight_summary"`
	GeneratedAt          time.Time                    `json:"generated_at"`
}

type EraComparisonItem struct {
	MaterialCode       string  `json:"material_code"`
	MaterialName       string  `json:"material_name"`
	Era                string  `json:"era"`
	PredictedLifeHours float64 `json:"predicted_life_hours"`
	PredictedLifeYears float64 `json:"predicted_life_years"`
	WearRateUmPerHour  float64 `json:"wear_rate_um_per_hour"`
	EraRank            int     `json:"era_rank"`
}

type VirtualMaintenanceAction struct {
	SessionID       string  `json:"session_id"`
	BearingID       int     `json:"bearing_id"`
	ActionType      string  `json:"action_type"`
	MaterialCode    *string `json:"material_code,omitempty"`
	LubricantCode   *string `json:"lubricant_code,omitempty"`
	LubricantAmount *float64 `json:"lubricant_amount_ml,omitempty"`
	OperatorName    *string `json:"operator_name,omitempty"`
}

type MaintenanceEffectPreview struct {
	BearingID           int     `json:"bearing_id"`
	CurrentWearUm       float64 `json:"current_wear_um"`
	CurrentLifeHours    float64 `json:"current_predicted_life_hours"`
	ProjectedLifeHours  float64 `json:"projected_life_hours"`
	LifeExtensionHours  float64 `json:"life_extension_hours"`
	LifeExtensionPct    float64 `json:"life_extension_percent"`
	NewWearRateUmHour   float64 `json:"new_wear_rate_um_per_hour"`
	OldWearRateUmHour   float64 `json:"old_wear_rate_um_per_hour"`
	ActionSummary       string  `json:"action_summary"`
	MaintenanceCostHint *string `json:"maintenance_cost_hint,omitempty"`
}

