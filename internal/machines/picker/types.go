package picker

import (
	"time"

	"github.com/sebastiankruger/shopfloor-simulator/internal/core"
)

// PickerPhase represents the phase within a pick-and-place cycle
type PickerPhase int

const (
	PhaseIdle PickerPhase = iota
	PhaseMoveToPickup   // Moving to pickup position above forming machine
	PhaseApproachPickup // Descending to pickup
	PhaseGrip           // Gripper closing on part
	PhaseRetractPickup  // Lifting part from pickup
	PhaseMoveToPlace    // Moving to place position above welder
	PhaseApproachPlace  // Descending to place
	PhaseRelease        // Gripper opening to release part
	PhaseRetractPlace   // Lifting away from placed part
)

func (p PickerPhase) String() string {
	switch p {
	case PhaseIdle:
		return "Idle"
	case PhaseMoveToPickup:
		return "MoveToPickup"
	case PhaseApproachPickup:
		return "ApproachPickup"
	case PhaseGrip:
		return "Grip"
	case PhaseRetractPickup:
		return "RetractPickup"
	case PhaseMoveToPlace:
		return "MoveToPlace"
	case PhaseApproachPlace:
		return "ApproachPlace"
	case PhaseRelease:
		return "Release"
	case PhaseRetractPlace:
		return "RetractPlace"
	default:
		return "Unknown"
	}
}

// GripperState represents the state of the gripper
type GripperState int

const (
	GripperOpen GripperState = iota
	GripperClosing
	GripperClosed
	GripperOpening
)

func (g GripperState) String() string {
	switch g {
	case GripperOpen:
		return "Open"
	case GripperClosing:
		return "Closing"
	case GripperClosed:
		return "Closed"
	case GripperOpening:
		return "Opening"
	default:
		return "Unknown"
	}
}

// ErrorCode represents picker-specific error types
type ErrorCode string

const (
	ErrorNone           ErrorCode = ""
	ErrorGripperFault   ErrorCode = "P001"
	ErrorCollision      ErrorCode = "P002"
	ErrorPositionFault  ErrorCode = "P003"
	ErrorPartDropped    ErrorCode = "P004"
	ErrorServoOverload  ErrorCode = "P005"
	ErrorEmergencyStop  ErrorCode = "P006"
)

// GetErrorInfo returns error details for a given picker error code
func GetErrorInfo(code ErrorCode) (message string, minDuration, maxDuration time.Duration) {
	switch code {
	case ErrorGripperFault:
		return "Gripper mechanism fault", 30 * time.Second, 1 * time.Minute
	case ErrorCollision:
		return "Collision detected", 1 * time.Minute, 3 * time.Minute
	case ErrorPositionFault:
		return "Position feedback error", 45 * time.Second, 2 * time.Minute
	case ErrorPartDropped:
		return "Part dropped during transfer", 30 * time.Second, 1 * time.Minute
	case ErrorServoOverload:
		return "Servo motor overload", 1 * time.Minute, 2 * time.Minute
	case ErrorEmergencyStop:
		return "Emergency stop activated", 20 * time.Second, 1 * time.Minute
	default:
		return "", 0, 0
	}
}

// AllErrorCodes returns all possible picker error codes
func AllErrorCodes() []ErrorCode {
	return []ErrorCode{
		ErrorGripperFault,
		ErrorCollision,
		ErrorPositionFault,
		ErrorPartDropped,
		ErrorServoOverload,
		ErrorEmergencyStop,
	}
}

// Position3D represents a 3D position
type Position3D struct {
	X float64 `json:"x"` // mm
	Y float64 `json:"y"` // mm
	Z float64 `json:"z"` // mm
}

// JointAngles represents 6-axis robot joint positions
type JointAngles struct {
	J1 float64 `json:"j1"` // degrees - base rotation
	J2 float64 `json:"j2"` // degrees - shoulder
	J3 float64 `json:"j3"` // degrees - elbow
	J4 float64 `json:"j4"` // degrees - wrist 1
	J5 float64 `json:"j5"` // degrees - wrist 2
	J6 float64 `json:"j6"` // degrees - wrist 3 (tool rotation)
}

// PickerData holds picker-specific timeseries data
type PickerData struct {
	// TCP (Tool Center Point) position
	PositionX float64 `json:"positionX"` // mm
	PositionY float64 `json:"positionY"` // mm
	PositionZ float64 `json:"positionZ"` // mm

	// Speed
	Speed float64 `json:"speed"` // mm/s - TCP speed

	// Joint angles (optional, for detailed simulation)
	Joint1 float64 `json:"joint1"` // degrees
	Joint2 float64 `json:"joint2"` // degrees
	Joint3 float64 `json:"joint3"` // degrees
	Joint4 float64 `json:"joint4"` // degrees
	Joint5 float64 `json:"joint5"` // degrees
	Joint6 float64 `json:"joint6"` // degrees

	// Gripper
	GripperState    GripperState `json:"gripperState"`
	GripperPosition float64      `json:"gripperPosition"` // 0=open, 100=closed (%)
	GripForce       float64      `json:"gripForce"`       // N

	// Cycle info
	CycleCount int     `json:"cycleCount"`
	CycleTime  float64 `json:"cycleTime"` // current cycle time in seconds

	// State info
	State             core.MachineState `json:"state"`
	Phase             PickerPhase       `json:"phase"`
	GoodParts         int               `json:"goodParts"`
	ScrapParts        int               `json:"scrapParts"`
	CurrentOrderID    string            `json:"currentOrderId"`
	CurrentPartNumber string            `json:"currentPartNumber"`
	CycleProgress     float64           `json:"cycleProgress"` // 0-100%

	// Part tracking
	PartInGripper string `json:"partInGripper"` // Part ID or empty

	// Error info
	ErrorCode      string    `json:"errorCode"`
	ErrorMessage   string    `json:"errorMessage"`
	ErrorTimestamp time.Time `json:"errorTimestamp,omitempty"`

	// Timestamp
	Timestamp time.Time `json:"timestamp"`
}

// ToMap converts PickerData to a map for OPC UA updates
func (pd *PickerData) ToMap() map[string]interface{} {
	return map[string]interface{}{
		"PositionX":         pd.PositionX,
		"PositionY":         pd.PositionY,
		"PositionZ":         pd.PositionZ,
		"Speed":             pd.Speed,
		"Joint1":            pd.Joint1,
		"Joint2":            pd.Joint2,
		"Joint3":            pd.Joint3,
		"Joint4":            pd.Joint4,
		"Joint5":            pd.Joint5,
		"Joint6":            pd.Joint6,
		"GripperState":      int32(pd.GripperState),
		"GripperPosition":   pd.GripperPosition,
		"GripForce":         pd.GripForce,
		"CycleCount":        int32(pd.CycleCount),
		"CycleTime":         pd.CycleTime,
		"State":             int32(pd.State),
		"GoodParts":         int32(pd.GoodParts),
		"ScrapParts":        int32(pd.ScrapParts),
		"CurrentOrderId":    pd.CurrentOrderID,
		"CurrentPartNumber": pd.CurrentPartNumber,
		"CycleProgress":     pd.CycleProgress,
		"PartInGripper":     pd.PartInGripper,
		"ErrorCode":         pd.ErrorCode,
		"ErrorMessage":      pd.ErrorMessage,
	}
}

// PickerConfig holds picker robot specific configuration
type PickerConfig struct {
	// Workspace limits
	MaxReachX float64 // mm
	MaxReachY float64 // mm
	MaxReachZ float64 // mm

	// Speed limits
	MaxSpeed       float64 // mm/s - TCP speed
	MaxJointSpeed  float64 // deg/s
	AccelerationG  float64 // g (acceleration in multiples of gravity)

	// Gripper
	MaxGripForce float64 // N
	GripTime     time.Duration // Time to close/open gripper

	// Positions
	HomePosition   Position3D
	PickupPosition Position3D
	PlacePosition  Position3D
	SafeZ          float64 // Safe Z height for horizontal moves

	// Phase timing (fractions of cycle time)
	MoveToPickupFraction   float64 // 0.15
	ApproachPickupFraction float64 // 0.10
	GripFraction           float64 // 0.05
	RetractPickupFraction  float64 // 0.10
	MoveToPlaceFraction    float64 // 0.20
	ApproachPlaceFraction  float64 // 0.10
	ReleaseFraction        float64 // 0.05
	RetractPlaceFraction   float64 // 0.10
	// Remaining ~0.15 = idle/wait time
}

// DefaultPickerConfig returns default picker robot parameters
func DefaultPickerConfig() PickerConfig {
	return PickerConfig{
		// Workspace
		MaxReachX: 1500.0, // mm
		MaxReachY: 1500.0, // mm
		MaxReachZ: 1000.0, // mm

		// Speed
		MaxSpeed:      500.0, // mm/s
		MaxJointSpeed: 180.0, // deg/s
		AccelerationG: 1.5,   // 1.5g acceleration

		// Gripper
		MaxGripForce: 100.0,                   // N
		GripTime:     300 * time.Millisecond,  // 300ms to grip/release

		// Positions (typical layout: forming machine on left, welder on right)
		HomePosition:   Position3D{X: 500, Y: 0, Z: 800},
		PickupPosition: Position3D{X: 100, Y: 300, Z: 200},  // Over forming output
		PlacePosition:  Position3D{X: 900, Y: 300, Z: 200},  // Over welder input
		SafeZ:          600.0,                                // Safe travel height

		// Phase timing
		MoveToPickupFraction:   0.15,
		ApproachPickupFraction: 0.08,
		GripFraction:           0.05,
		RetractPickupFraction:  0.08,
		MoveToPlaceFraction:    0.20,
		ApproachPlaceFraction:  0.08,
		ReleaseFraction:        0.05,
		RetractPlaceFraction:   0.08,
		// Remaining 0.23 = buffer/idle
	}
}

// PickerState holds the complete internal state of the picker robot
type PickerState struct {
	// Current phase within pick-place cycle
	Phase          PickerPhase
	PhaseStartedAt time.Time

	// Cycle counter
	CycleCount int

	// Current position
	CurrentPosition Position3D
	TargetPosition  Position3D

	// Joint angles
	CurrentJoints JointAngles

	// Gripper state
	Gripper         GripperState
	GripperPosition float64 // 0-100%

	// Part being held
	HeldPartID string
	HeldPart   *core.Part
}
