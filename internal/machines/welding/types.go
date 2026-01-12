package welding

import (
	"time"

	"github.com/sebastiankruger/shopfloor-simulator/internal/core"
)

// WeldPhase represents the phase within a welding cycle
type WeldPhase int

const (
	PhaseOff WeldPhase = iota
	PhaseRampUp
	PhaseSteady
	PhaseRampDown
)

func (p WeldPhase) String() string {
	switch p {
	case PhaseOff:
		return "Off"
	case PhaseRampUp:
		return "RampUp"
	case PhaseSteady:
		return "Steady"
	case PhaseRampDown:
		return "RampDown"
	default:
		return "Unknown"
	}
}

// ErrorCode represents welding-specific error types
type ErrorCode string

const (
	ErrorNone           ErrorCode = ""
	ErrorWireFeedJam    ErrorCode = "E001"
	ErrorGasFlowFault   ErrorCode = "E002"
	ErrorArcFault       ErrorCode = "E003"
	ErrorRobotCollision ErrorCode = "E004"
	ErrorQualityReject  ErrorCode = "E005"
)

// GetErrorInfo returns error details for a given welding error code
func GetErrorInfo(code ErrorCode) (message string, minDuration, maxDuration time.Duration) {
	switch code {
	case ErrorWireFeedJam:
		return "Wire feed jam detected", 5 * time.Minute, 10 * time.Minute
	case ErrorGasFlowFault:
		return "Gas flow fault", 2 * time.Minute, 5 * time.Minute
	case ErrorArcFault:
		return "Arc fault detected", 1 * time.Minute, 3 * time.Minute
	case ErrorRobotCollision:
		return "Robot collision detected", 15 * time.Minute, 30 * time.Minute
	case ErrorQualityReject:
		return "Quality reject", 1 * time.Minute, 2 * time.Minute
	default:
		return "", 0, 0
	}
}

// AllErrorCodes returns all possible welding error codes
func AllErrorCodes() []ErrorCode {
	return []ErrorCode{
		ErrorWireFeedJam,
		ErrorGasFlowFault,
		ErrorArcFault,
		ErrorRobotCollision,
		ErrorQualityReject,
	}
}

// WeldingData holds welding-specific timeseries data
type WeldingData struct {
	// Welding parameters
	WeldingCurrent float64 `json:"weldingCurrent"` // Amps
	Voltage        float64 `json:"voltage"`        // Volts
	WireFeedSpeed  float64 `json:"wireFeedSpeed"`  // m/min
	GasFlow        float64 `json:"gasFlow"`        // l/min
	TravelSpeed    float64 `json:"travelSpeed"`    // mm/s
	ArcTime        float64 `json:"arcTime"`        // cumulative seconds

	// Position
	PositionX  float64 `json:"positionX"`  // mm
	PositionY  float64 `json:"positionY"`  // mm
	PositionZ  float64 `json:"positionZ"`  // mm
	TorchAngle float64 `json:"torchAngle"` // degrees

	// State info
	State             core.MachineState `json:"state"`
	Phase             WeldPhase         `json:"phase"`
	GoodParts         int               `json:"goodParts"`
	ScrapParts        int               `json:"scrapParts"`
	CurrentOrderID    string            `json:"currentOrderId"`
	CurrentPartNumber string            `json:"currentPartNumber"`
	CycleProgress     float64           `json:"cycleProgress"` // 0-100%

	// Error info
	ErrorCode      string    `json:"errorCode"`
	ErrorMessage   string    `json:"errorMessage"`
	ErrorTimestamp time.Time `json:"errorTimestamp,omitempty"`

	// Timestamp
	Timestamp time.Time `json:"timestamp"`
}

// ToMap converts WeldingData to a map for OPC UA updates
func (wd *WeldingData) ToMap() map[string]interface{} {
	return map[string]interface{}{
		"WeldingCurrent":    wd.WeldingCurrent,
		"Voltage":           wd.Voltage,
		"WireFeedSpeed":     wd.WireFeedSpeed,
		"GasFlow":           wd.GasFlow,
		"TravelSpeed":       wd.TravelSpeed,
		"ArcTime":           wd.ArcTime,
		"Position.X":        wd.PositionX,
		"Position.Y":        wd.PositionY,
		"Position.Z":        wd.PositionZ,
		"TorchAngle":        wd.TorchAngle,
		"State":             int32(wd.State),
		"GoodParts":         int32(wd.GoodParts),
		"ScrapParts":        int32(wd.ScrapParts),
		"CurrentOrderId":    wd.CurrentOrderID,
		"CurrentPartNumber": wd.CurrentPartNumber,
		"CycleProgress":     wd.CycleProgress,
		"ErrorCode":         wd.ErrorCode,
		"ErrorMessage":      wd.ErrorMessage,
	}
}

// WeldingConfig holds welding robot specific configuration
type WeldingConfig struct {
	// Target welding parameters
	TargetCurrent       float64 // Amps
	TargetVoltage       float64 // Volts
	TargetWireFeedSpeed float64 // m/min
	TargetGasFlow       float64 // l/min
	TargetTravelSpeed   float64 // mm/s

	// Path simulation
	WeldPathLength float64 // mm
}

// DefaultWeldingConfig returns default welding parameters
func DefaultWeldingConfig() WeldingConfig {
	return WeldingConfig{
		// Default values for mild steel, 0.035-0.045" wire
		TargetCurrent:       200.0, // Amps
		TargetVoltage:       24.0,  // Volts
		TargetWireFeedSpeed: 9.6,   // m/min (~380 IPM)
		TargetGasFlow:       15.0,  // l/min (~32 CFH)
		TargetTravelSpeed:   10.0,  // mm/s
		WeldPathLength:      500.0, // mm total weld path
	}
}

// WeldingState holds the complete internal state of the welding robot
type WeldingState struct {
	// Current phase within welding cycle
	WeldPhase      WeldPhase
	PhaseStartedAt time.Time

	// Arc time accumulator
	ArcTime float64

	// Timeseries generation state
	WeldPathProgress float64
}
