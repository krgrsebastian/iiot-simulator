package simulator

import (
	"time"

	"github.com/sebastiankruger/shopfloor-simulator/internal/core"
	"github.com/sebastiankruger/shopfloor-simulator/internal/machines/welding"
)

// Re-export MachineState from core for backward compatibility
type MachineState = core.MachineState

// Machine state constants - re-exported from core
const (
	StateIdle          = core.StateIdle
	StateSetup         = core.StateSetup
	StateRunning       = core.StateRunning
	StatePlannedStop   = core.StatePlannedStop
	StateUnplannedStop = core.StateUnplannedStop
)

// WeldPhase - re-exported from welding package
type WeldPhase = welding.WeldPhase

// Weld phase constants - re-exported from welding
const (
	PhaseOff      = welding.PhaseOff
	PhaseRampUp   = welding.PhaseRampUp
	PhaseSteady   = welding.PhaseSteady
	PhaseRampDown = welding.PhaseRampDown
)

// ErrorCode - re-exported from welding package
type ErrorCode = welding.ErrorCode

// Error code constants - re-exported from welding
const (
	ErrorNone           = welding.ErrorNone
	ErrorWireFeedJam    = welding.ErrorWireFeedJam
	ErrorGasFlowFault   = welding.ErrorGasFlowFault
	ErrorArcFault       = welding.ErrorArcFault
	ErrorRobotCollision = welding.ErrorRobotCollision
	ErrorQualityReject  = welding.ErrorQualityReject
)

// ErrorInfo - re-exported from core
type ErrorInfo = core.ErrorInfo

// GetErrorInfo wraps welding.GetErrorInfo for backward compatibility
func GetErrorInfo(code ErrorCode) (message string, minDuration, maxDuration time.Duration) {
	return welding.GetErrorInfo(code)
}

// ProductionOrder - re-exported from core
type ProductionOrder = core.ProductionOrder

// Order status constants - re-exported from core
const (
	OrderStatusQueued     = core.OrderStatusQueued
	OrderStatusInProgress = core.OrderStatusInProgress
	OrderStatusCompleted  = core.OrderStatusCompleted
	OrderStatusCancelled  = core.OrderStatusCancelled
)

// Shift - re-exported from core
type Shift = core.Shift

// PlannedBreak - re-exported from core
type PlannedBreak = core.PlannedBreak

// Shift status constants - re-exported from core
const (
	ShiftStatusActive   = core.ShiftStatusActive
	ShiftStatusEnded    = core.ShiftStatusEnded
	ShiftStatusUpcoming = core.ShiftStatusUpcoming
)

// PartDefinition - re-exported from core
type PartDefinition = core.PartDefinition

// TimeseriesData holds all current timeseries values (legacy compatibility)
type TimeseriesData struct {
	// Welding parameters
	WeldingCurrent float64
	Voltage        float64
	WireFeedSpeed  float64
	GasFlow        float64
	TravelSpeed    float64
	ArcTime        float64

	// Position
	PositionX  float64
	PositionY  float64
	PositionZ  float64
	TorchAngle float64

	// State info
	State             MachineState
	GoodParts         int
	ScrapParts        int
	CurrentOrderID    string
	CurrentPartNumber string
	CycleProgress     float64

	// Error info
	ErrorCode      string
	ErrorMessage   string
	ErrorTimestamp time.Time

	// Timestamp
	Timestamp time.Time
}

// FromWeldingData converts WeldingData to TimeseriesData for backward compatibility
func FromWeldingData(wd *welding.WeldingData) *TimeseriesData {
	return &TimeseriesData{
		WeldingCurrent:    wd.WeldingCurrent,
		Voltage:           wd.Voltage,
		WireFeedSpeed:     wd.WireFeedSpeed,
		GasFlow:           wd.GasFlow,
		TravelSpeed:       wd.TravelSpeed,
		ArcTime:           wd.ArcTime,
		PositionX:         wd.PositionX,
		PositionY:         wd.PositionY,
		PositionZ:         wd.PositionZ,
		TorchAngle:        wd.TorchAngle,
		State:             wd.State,
		GoodParts:         wd.GoodParts,
		ScrapParts:        wd.ScrapParts,
		CurrentOrderID:    wd.CurrentOrderID,
		CurrentPartNumber: wd.CurrentPartNumber,
		CycleProgress:     wd.CycleProgress,
		ErrorCode:         wd.ErrorCode,
		ErrorMessage:      wd.ErrorMessage,
		ErrorTimestamp:    wd.ErrorTimestamp,
		Timestamp:         wd.Timestamp,
	}
}

// SimulatorState holds the complete state of the simulator (legacy compatibility)
type SimulatorState struct {
	// Current state
	State     MachineState
	WeldPhase WeldPhase

	// Timing
	StateEnteredAt time.Time
	CycleStartedAt time.Time
	PhaseStartedAt time.Time
	LastPublishAt  time.Time

	// Current work
	CurrentOrder *ProductionOrder
	CurrentShift *Shift

	// Counters (reset per shift)
	GoodParts  int
	ScrapParts int
	ArcTime    float64

	// Error state
	CurrentError *ErrorInfo

	// Order queue
	OrderQueue []*ProductionOrder

	// Timeseries state (for colored noise)
	LastCurrent       float64
	LastVoltage       float64
	ColoredNoiseState float64
}
