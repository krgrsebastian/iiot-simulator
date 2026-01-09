package simulator

import (
	"time"
)

// MachineState represents the current state of the welding robot
type MachineState int

const (
	StateIdle MachineState = iota
	StateSetup
	StateRunning
	StatePlannedStop
	StateUnplannedStop
)

func (s MachineState) String() string {
	switch s {
	case StateIdle:
		return "Idle"
	case StateSetup:
		return "Setup"
	case StateRunning:
		return "Running"
	case StatePlannedStop:
		return "PlannedStop"
	case StateUnplannedStop:
		return "UnplannedStop"
	default:
		return "Unknown"
	}
}

// WeldPhase represents the phase within a welding cycle
type WeldPhase int

const (
	PhaseOff WeldPhase = iota
	PhaseRampUp
	PhaseSteady
	PhaseRampDown
)

// ErrorCode represents different error types
type ErrorCode string

const (
	ErrorNone           ErrorCode = ""
	ErrorWireFeedJam    ErrorCode = "E001"
	ErrorGasFlowFault   ErrorCode = "E002"
	ErrorArcFault       ErrorCode = "E003"
	ErrorRobotCollision ErrorCode = "E004"
	ErrorQualityReject  ErrorCode = "E005"
)

// ErrorInfo contains information about the current error
type ErrorInfo struct {
	Code        ErrorCode
	Message     string
	OccurredAt  time.Time
	ExpectedEnd time.Time
}

// GetErrorInfo returns error details for a given error code
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

// ProductionOrder represents a manufacturing order
type ProductionOrder struct {
	OrderID             string    `json:"orderId"`
	PartNumber          string    `json:"partNumber"`
	PartDescription     string    `json:"partDescription"`
	Quantity            int       `json:"quantity"`
	QuantityCompleted   int       `json:"quantityCompleted"`
	QuantityScrap       int       `json:"quantityScrap"`
	DueDate             time.Time `json:"dueDate"`
	Customer            string    `json:"customer"`
	Priority            int       `json:"priority"`
	Status              string    `json:"status"`
	StartedAt           time.Time `json:"startedAt,omitempty"`
	EstimatedCompletion time.Time `json:"estimatedCompletion,omitempty"`
}

// Order status constants
const (
	OrderStatusQueued     = "QUEUED"
	OrderStatusInProgress = "IN_PROGRESS"
	OrderStatusCompleted  = "COMPLETED"
	OrderStatusCancelled  = "CANCELLED"
)

// Shift represents a work shift
type Shift struct {
	ShiftID       string        `json:"shiftId"`
	ShiftName     string        `json:"shiftName"`
	ShiftNumber   int           `json:"shiftNumber"`
	StartTime     time.Time     `json:"startTime"`
	EndTime       time.Time     `json:"endTime"`
	WorkCenterID  string        `json:"workCenterId"`
	PlannedBreaks []PlannedBreak `json:"plannedBreaks"`
	Status        string        `json:"status"`
}

// PlannedBreak represents a scheduled break within a shift
type PlannedBreak struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
	Type  string    `json:"type"` // "break" or "lunch"
}

// Shift status constants
const (
	ShiftStatusActive   = "ACTIVE"
	ShiftStatusEnded    = "ENDED"
	ShiftStatusUpcoming = "UPCOMING"
)

// PartDefinition defines a part type that can be produced
type PartDefinition struct {
	PartNumber  string
	Description string
	CycleTime   time.Duration
}

// TimeseriesData holds all current timeseries values
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

// SimulatorState holds the complete state of the simulator
type SimulatorState struct {
	// Current state
	State     MachineState
	WeldPhase WeldPhase

	// Timing
	StateEnteredAt    time.Time
	CycleStartedAt    time.Time
	PhaseStartedAt    time.Time
	LastPublishAt     time.Time

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
	LastCurrent float64
	LastVoltage float64
	ColoredNoiseState float64
}
