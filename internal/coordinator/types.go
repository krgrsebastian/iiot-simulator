package coordinator

import (
	"time"

	"github.com/sebastiankruger/shopfloor-simulator/internal/core"
)

// LineState represents the overall production line state
type LineState string

const (
	LineStateRunning LineState = "Running"
	LineStateStopped LineState = "Stopped"
	LineStateError   LineState = "Error"
	LineStateSetup   LineState = "Setup"
)

// LineData holds production line timeseries data for OPC UA
type LineData struct {
	// Line state
	LineState string `json:"lineState"`

	// Work in progress
	WIPCount int `json:"wipCount"` // Parts currently in the line

	// Throughput
	ThroughputPerHour float64 `json:"throughputPerHour"`

	// Bottleneck
	BottleneckMachine string `json:"bottleneckMachine"`

	// Counts
	TotalPartsCompleted int `json:"totalPartsCompleted"`
	TotalPartsScrap     int `json:"totalPartsScrap"`
	TotalPartsStarted   int `json:"totalPartsStarted"`

	// Per-station counts
	FormingCompleted int `json:"formingCompleted"`
	PickingCompleted int `json:"pickingCompleted"`
	WeldingCompleted int `json:"weldingCompleted"`

	// Buffer levels
	FormingBufferCount int `json:"formingBufferCount"`
	PickerBufferCount  int `json:"pickerBufferCount"` // Output of picker = input of welder

	// OEE components
	Availability float64 `json:"availability"` // 0-100%
	Performance  float64 `json:"performance"`  // 0-100%
	Quality      float64 `json:"quality"`      // 0-100%
	OEE          float64 `json:"oee"`          // 0-100%

	// Timing
	LineUptime      float64 `json:"lineUptime"`      // seconds
	LineDowntime    float64 `json:"lineDowntime"`    // seconds
	AverageCycleTime float64 `json:"averageCycleTime"` // seconds

	// Current order
	CurrentOrderID    string `json:"currentOrderId"`
	CurrentPartNumber string `json:"currentPartNumber"`
	OrderProgress     float64 `json:"orderProgress"` // 0-100%

	// Errors
	ActiveErrors   int    `json:"activeErrors"`
	LastErrorCode  string `json:"lastErrorCode"`
	LastErrorMachine string `json:"lastErrorMachine"`

	// Timestamp
	Timestamp time.Time `json:"timestamp"`
}

// ToMap converts LineData to a map for OPC UA updates
func (ld *LineData) ToMap() map[string]interface{} {
	return map[string]interface{}{
		"LineState":           ld.LineState,
		"WIPCount":            int32(ld.WIPCount),
		"ThroughputPerHour":   ld.ThroughputPerHour,
		"BottleneckMachine":   ld.BottleneckMachine,
		"TotalPartsCompleted": int32(ld.TotalPartsCompleted),
		"TotalPartsScrap":     int32(ld.TotalPartsScrap),
		"TotalPartsStarted":   int32(ld.TotalPartsStarted),
		"FormingCompleted":    int32(ld.FormingCompleted),
		"PickingCompleted":    int32(ld.PickingCompleted),
		"WeldingCompleted":    int32(ld.WeldingCompleted),
		"FormingBufferCount":  int32(ld.FormingBufferCount),
		"PickerBufferCount":   int32(ld.PickerBufferCount),
		"Availability":        ld.Availability,
		"Performance":         ld.Performance,
		"Quality":             ld.Quality,
		"OEE":                 ld.OEE,
		"LineUptime":          ld.LineUptime,
		"LineDowntime":        ld.LineDowntime,
		"AverageCycleTime":    ld.AverageCycleTime,
		"CurrentOrderId":      ld.CurrentOrderID,
		"CurrentPartNumber":   ld.CurrentPartNumber,
		"OrderProgress":       ld.OrderProgress,
		"ActiveErrors":        int32(ld.ActiveErrors),
		"LastErrorCode":       ld.LastErrorCode,
		"LastErrorMachine":    ld.LastErrorMachine,
	}
}

// LineConfig holds production line configuration
type LineConfig struct {
	// Machine names
	FormingMachineName string
	PickerRobotName    string
	SpotWelderName     string

	// Line name
	LineName string

	// Theoretical cycle times (for OEE calculation)
	TheoreticalCycleTime time.Duration

	// Buffer capacities
	FormingBufferCapacity int
	WelderBufferCapacity  int
}

// DefaultLineConfig returns default line configuration
func DefaultLineConfig() LineConfig {
	return LineConfig{
		FormingMachineName: "FormingMachine",
		PickerRobotName:    "PickerRobot",
		SpotWelderName:     "SpotWelder",
		LineName:           "ProductionLine",

		TheoreticalCycleTime: 60 * time.Second, // 60 parts per hour theoretical

		FormingBufferCapacity: 5,
		WelderBufferCapacity:  3,
	}
}

// MachineMetrics holds per-machine metrics for bottleneck analysis
type MachineMetrics struct {
	Name             string
	State            core.MachineState
	CycleCount       int
	GoodParts        int
	ScrapParts       int
	TotalUptime      time.Duration
	TotalDowntime    time.Duration
	CurrentCycleTime time.Duration
	AverageCycleTime time.Duration
	IsBottleneck     bool
}

// PartEvent represents an event in part flow
type PartEvent struct {
	PartID    string
	EventType PartEventType
	Machine   string
	Timestamp time.Time
	OrderID   string
}

// PartEventType represents types of part events
type PartEventType string

const (
	PartEventCreated        PartEventType = "CREATED"
	PartEventFormingComplete PartEventType = "FORMING_COMPLETE"
	PartEventPickedUp       PartEventType = "PICKED_UP"
	PartEventPlaced         PartEventType = "PLACED"
	PartEventWeldingComplete PartEventType = "WELDING_COMPLETE"
	PartEventScrapped       PartEventType = "SCRAPPED"
)
