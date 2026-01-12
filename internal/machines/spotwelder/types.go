package spotwelder

import (
	"time"

	"github.com/sebastiankruger/shopfloor-simulator/internal/core"
)

// SpotWelderPhase represents the phase within a spot welding cycle
type SpotWelderPhase int

const (
	PhaseIdle SpotWelderPhase = iota
	PhaseLoad       // Part loaded onto fixture
	PhaseClamp      // Fixtures clamping part
	PhasePreWeld    // Positioning weld stud
	PhaseWeld       // Welding current flowing
	PhaseHold       // Post-weld hold/cooling
	PhaseRelease    // Fixtures releasing
	PhaseUnload     // Part unloading
)

func (p SpotWelderPhase) String() string {
	switch p {
	case PhaseIdle:
		return "Idle"
	case PhaseLoad:
		return "Load"
	case PhaseClamp:
		return "Clamp"
	case PhasePreWeld:
		return "PreWeld"
	case PhaseWeld:
		return "Weld"
	case PhaseHold:
		return "Hold"
	case PhaseRelease:
		return "Release"
	case PhaseUnload:
		return "Unload"
	default:
		return "Unknown"
	}
}

// ErrorCode represents spot welder-specific error types
type ErrorCode string

const (
	ErrorNone            ErrorCode = ""
	ErrorWeldFault       ErrorCode = "S001"
	ErrorClampFault      ErrorCode = "S002"
	ErrorStudMisfeed     ErrorCode = "S003"
	ErrorOverheat        ErrorCode = "S004"
	ErrorCurrentFault    ErrorCode = "S005"
	ErrorQualityReject   ErrorCode = "S006"
)

// GetErrorInfo returns error details for a given spot welder error code
func GetErrorInfo(code ErrorCode) (message string, minDuration, maxDuration time.Duration) {
	switch code {
	case ErrorWeldFault:
		return "Weld quality fault detected", 30 * time.Second, 1 * time.Minute
	case ErrorClampFault:
		return "Clamp mechanism fault", 45 * time.Second, 2 * time.Minute
	case ErrorStudMisfeed:
		return "Stud feed mechanism jam", 30 * time.Second, 1 * time.Minute
	case ErrorOverheat:
		return "Electrode overheat protection", 1 * time.Minute, 3 * time.Minute
	case ErrorCurrentFault:
		return "Weld current out of range", 45 * time.Second, 90 * time.Second
	case ErrorQualityReject:
		return "Weld quality below threshold", 20 * time.Second, 45 * time.Second
	default:
		return "", 0, 0
	}
}

// AllErrorCodes returns all possible spot welder error codes
func AllErrorCodes() []ErrorCode {
	return []ErrorCode{
		ErrorWeldFault,
		ErrorClampFault,
		ErrorStudMisfeed,
		ErrorOverheat,
		ErrorCurrentFault,
		ErrorQualityReject,
	}
}

// SpotWelderData holds spot welder-specific timeseries data
type SpotWelderData struct {
	// Welding parameters
	WeldCurrent    float64 `json:"weldCurrent"`    // kA (kiloamps)
	WeldVoltage    float64 `json:"weldVoltage"`    // V
	WeldTime       float64 `json:"weldTime"`       // ms (current weld time)
	WeldEnergy     float64 `json:"weldEnergy"`     // J (joules)

	// Force and pressure
	ElectrodeForce float64 `json:"electrodeForce"` // kN
	ClampForce     float64 `json:"clampForce"`     // kN

	// Temperature
	ElectrodeTemp  float64 `json:"electrodeTemp"`  // °C
	PartTemp       float64 `json:"partTemp"`       // °C

	// Counts
	WeldCount   int `json:"weldCount"`   // Welds in current part
	TotalWelds  int `json:"totalWelds"`  // Total welds performed
	CycleCount  int `json:"cycleCount"`  // Parts completed

	// Cycle info
	CycleTime float64 `json:"cycleTime"` // current cycle time in seconds

	// State info
	State             core.MachineState `json:"state"`
	Phase             SpotWelderPhase   `json:"phase"`
	GoodParts         int               `json:"goodParts"`
	ScrapParts        int               `json:"scrapParts"`
	CurrentOrderID    string            `json:"currentOrderId"`
	CurrentPartNumber string            `json:"currentPartNumber"`
	CycleProgress     float64           `json:"cycleProgress"` // 0-100%

	// Part tracking
	CurrentPartID string `json:"currentPartId"`

	// Electrode wear
	ElectrodeWear float64 `json:"electrodeWear"` // 0-100% (100 = needs replacement)

	// Error info
	ErrorCode      string    `json:"errorCode"`
	ErrorMessage   string    `json:"errorMessage"`
	ErrorTimestamp time.Time `json:"errorTimestamp,omitempty"`

	// Timestamp
	Timestamp time.Time `json:"timestamp"`
}

// ToMap converts SpotWelderData to a map for OPC UA updates
func (sd *SpotWelderData) ToMap() map[string]interface{} {
	return map[string]interface{}{
		"WeldCurrent":       sd.WeldCurrent,
		"WeldVoltage":       sd.WeldVoltage,
		"WeldTime":          sd.WeldTime,
		"WeldEnergy":        sd.WeldEnergy,
		"ElectrodeForce":    sd.ElectrodeForce,
		"ClampForce":        sd.ClampForce,
		"ElectrodeTemp":     sd.ElectrodeTemp,
		"PartTemp":          sd.PartTemp,
		"WeldCount":         int32(sd.WeldCount),
		"TotalWelds":        int32(sd.TotalWelds),
		"CycleCount":        int32(sd.CycleCount),
		"CycleTime":         sd.CycleTime,
		"State":             int32(sd.State),
		"GoodParts":         int32(sd.GoodParts),
		"ScrapParts":        int32(sd.ScrapParts),
		"CurrentOrderId":    sd.CurrentOrderID,
		"CurrentPartNumber": sd.CurrentPartNumber,
		"CycleProgress":     sd.CycleProgress,
		"CurrentPartId":     sd.CurrentPartID,
		"ElectrodeWear":     sd.ElectrodeWear,
		"ErrorCode":         sd.ErrorCode,
		"ErrorMessage":      sd.ErrorMessage,
	}
}

// SpotWelderConfig holds spot welder specific configuration
type SpotWelderConfig struct {
	// Welding parameters
	TargetCurrent    float64       // kA
	TargetVoltage    float64       // V
	WeldDuration     time.Duration // Duration of each weld
	WeldsPerPart     int           // Number of studs per part

	// Force parameters
	MaxElectrodeForce float64 // kN
	MaxClampForce     float64 // kN

	// Temperature limits
	MaxElectrodeTemp float64 // °C

	// Phase timing (fractions of cycle time)
	LoadFraction    float64 // 0.10
	ClampFraction   float64 // 0.10
	PreWeldFraction float64 // 0.05
	WeldFraction    float64 // 0.35 (includes all welds)
	HoldFraction    float64 // 0.15
	ReleaseFraction float64 // 0.10
	UnloadFraction  float64 // 0.15

	// Electrode
	ElectrodeLifeWelds int // Welds before electrode replacement needed

	// Input buffer
	InputBufferCapacity int
}

// DefaultSpotWelderConfig returns default spot welder parameters
func DefaultSpotWelderConfig() SpotWelderConfig {
	return SpotWelderConfig{
		// Welding parameters (typical for steel: 6-10 kA, 3-12 V)
		TargetCurrent:  8.0,                      // kA (typical for steel sheet)
		TargetVoltage:  6.0,                      // V (typical secondary voltage)
		WeldDuration:   200 * time.Millisecond,   // 200ms per weld
		WeldsPerPart:   4,                        // 4 studs per rail

		// Force
		MaxElectrodeForce: 3.0, // kN
		MaxClampForce:     5.0, // kN

		// Temperature
		MaxElectrodeTemp: 400.0, // °C

		// Phase timing
		LoadFraction:    0.10,
		ClampFraction:   0.10,
		PreWeldFraction: 0.05,
		WeldFraction:    0.35, // Includes all 4 welds
		HoldFraction:    0.15,
		ReleaseFraction: 0.10,
		UnloadFraction:  0.15,

		// Electrode
		ElectrodeLifeWelds: 5000, // Replace after 5000 welds

		InputBufferCapacity: 3,
	}
}

// SpotWelderState holds the complete internal state of the spot welder
type SpotWelderState struct {
	// Current phase within welding cycle
	Phase          SpotWelderPhase
	PhaseStartedAt time.Time

	// Cycle counter
	CycleCount int

	// Weld tracking
	WeldsInCurrentPart int
	TotalWelds         int

	// Electrode wear
	ElectrodeWeldCount int

	// Current part
	CurrentPartID string
	CurrentPart   *core.Part

	// Input buffer (parts from picker)
	InputBuffer *core.PartBuffer

	// Temperatures (with thermal inertia)
	ElectrodeTemp float64
	PartTemp      float64
}
