package forming

import (
	"time"

	"github.com/sebastiankruger/shopfloor-simulator/internal/core"
)

// FormingPhase represents the phase within a forming cycle
type FormingPhase int

const (
	PhaseIdle FormingPhase = iota
	PhaseLoad      // Sheet metal loading
	PhaseForm      // Ram descending, forming
	PhaseHold      // Pressure maintained
	PhaseEject     // Part ejection
	PhaseRaise     // Ram returning to top
)

func (p FormingPhase) String() string {
	switch p {
	case PhaseIdle:
		return "Idle"
	case PhaseLoad:
		return "Load"
	case PhaseForm:
		return "Form"
	case PhaseHold:
		return "Hold"
	case PhaseEject:
		return "Eject"
	case PhaseRaise:
		return "Raise"
	default:
		return "Unknown"
	}
}

// ErrorCode represents forming-specific error types
type ErrorCode string

const (
	ErrorNone             ErrorCode = ""
	ErrorSheetMisfeed     ErrorCode = "F001"
	ErrorHydraulicFault   ErrorCode = "F002"
	ErrorOverpressure     ErrorCode = "F003"
	ErrorRamStuck         ErrorCode = "F004"
	ErrorQualityReject    ErrorCode = "F005"
	ErrorTemperatureFault ErrorCode = "F006"
)

// GetErrorInfo returns error details for a given forming error code
func GetErrorInfo(code ErrorCode) (message string, minDuration, maxDuration time.Duration) {
	switch code {
	case ErrorSheetMisfeed:
		return "Sheet metal misfeed detected", 30 * time.Second, 1 * time.Minute
	case ErrorHydraulicFault:
		return "Hydraulic system fault", 1 * time.Minute, 3 * time.Minute
	case ErrorOverpressure:
		return "Overpressure detected", 45 * time.Second, 2 * time.Minute
	case ErrorRamStuck:
		return "Ram stuck in position", 2 * time.Minute, 5 * time.Minute
	case ErrorQualityReject:
		return "Quality reject - forming defect", 15 * time.Second, 45 * time.Second
	case ErrorTemperatureFault:
		return "Temperature out of range", 1 * time.Minute, 2 * time.Minute
	default:
		return "", 0, 0
	}
}

// AllErrorCodes returns all possible forming error codes
func AllErrorCodes() []ErrorCode {
	return []ErrorCode{
		ErrorSheetMisfeed,
		ErrorHydraulicFault,
		ErrorOverpressure,
		ErrorRamStuck,
		ErrorQualityReject,
		ErrorTemperatureFault,
	}
}

// FormingData holds forming-specific timeseries data
type FormingData struct {
	// Process parameters
	Temperature   float64 `json:"temperature"`   // °C (die temperature)
	Pressure      float64 `json:"pressure"`      // bar (hydraulic pressure)
	FormingForce  float64 `json:"formingForce"`  // kN
	RamPosition   float64 `json:"ramPosition"`   // mm (0=top, 500=bottom)
	RamSpeed      float64 `json:"ramSpeed"`      // mm/s
	DieTemperature float64 `json:"dieTemperature"` // °C (die surface temp)

	// Cycle info
	CycleCount  int     `json:"cycleCount"`
	CycleTime   float64 `json:"cycleTime"`   // current cycle time in seconds

	// State info
	State             core.MachineState `json:"state"`
	Phase             FormingPhase      `json:"phase"`
	GoodParts         int               `json:"goodParts"`
	ScrapParts        int               `json:"scrapParts"`
	CurrentOrderID    string            `json:"currentOrderId"`
	CurrentPartNumber string            `json:"currentPartNumber"`
	CycleProgress     float64           `json:"cycleProgress"` // 0-100%

	// Output buffer
	OutputBufferCount int    `json:"outputBufferCount"`
	CurrentPartID     string `json:"currentPartId"`

	// Error info
	ErrorCode      string    `json:"errorCode"`
	ErrorMessage   string    `json:"errorMessage"`
	ErrorTimestamp time.Time `json:"errorTimestamp,omitempty"`

	// Timestamp
	Timestamp time.Time `json:"timestamp"`
}

// ToMap converts FormingData to a map for OPC UA updates
func (fd *FormingData) ToMap() map[string]interface{} {
	return map[string]interface{}{
		"Temperature":       fd.Temperature,
		"Pressure":          fd.Pressure,
		"FormingForce":      fd.FormingForce,
		"RamPosition":       fd.RamPosition,
		"RamSpeed":          fd.RamSpeed,
		"DieTemperature":    fd.DieTemperature,
		"CycleCount":        int32(fd.CycleCount),
		"CycleTime":         fd.CycleTime,
		"State":             int32(fd.State),
		"GoodParts":         int32(fd.GoodParts),
		"ScrapParts":        int32(fd.ScrapParts),
		"CurrentOrderId":    fd.CurrentOrderID,
		"CurrentPartNumber": fd.CurrentPartNumber,
		"CycleProgress":     fd.CycleProgress,
		"OutputBufferCount": int32(fd.OutputBufferCount),
		"CurrentPartId":     fd.CurrentPartID,
		"ErrorCode":         fd.ErrorCode,
		"ErrorMessage":      fd.ErrorMessage,
	}
}

// FormingConfig holds forming machine specific configuration
type FormingConfig struct {
	// Process parameters
	TargetTemperature float64 // °C (die temperature)
	MaxPressure       float64 // bar
	MaxFormingForce   float64 // kN
	RamTravel         float64 // mm (total travel distance)
	MaxRamSpeed       float64 // mm/s

	// Phase timing (fractions of cycle time)
	LoadFraction  float64 // 0.10 = 10% of cycle
	FormFraction  float64 // 0.40 = 40% of cycle
	HoldFraction  float64 // 0.15 = 15% of cycle
	EjectFraction float64 // 0.15 = 15% of cycle
	RaiseFraction float64 // 0.20 = 20% of cycle

	// Output buffer
	OutputBufferCapacity int
}

// DefaultFormingConfig returns default forming parameters
func DefaultFormingConfig() FormingConfig {
	return FormingConfig{
		// Default values for cold forming
		TargetTemperature: 45.0,  // °C (slightly above ambient due to friction)
		MaxPressure:       150.0, // bar
		MaxFormingForce:   250.0, // kN
		RamTravel:         400.0, // mm
		MaxRamSpeed:       80.0,  // mm/s

		// Phase timing
		LoadFraction:  0.10, // 10%
		FormFraction:  0.40, // 40%
		HoldFraction:  0.15, // 15%
		EjectFraction: 0.15, // 15%
		RaiseFraction: 0.20, // 20%

		OutputBufferCapacity: 5,
	}
}

// FormingState holds the complete internal state of the forming machine
type FormingState struct {
	// Current phase within forming cycle
	Phase          FormingPhase
	PhaseStartedAt time.Time

	// Cycle counter
	CycleCount int

	// Ram position tracking
	CurrentRamPosition float64

	// Output buffer
	OutputBuffer *core.PartBuffer

	// Current part being formed
	CurrentPartID string
}
