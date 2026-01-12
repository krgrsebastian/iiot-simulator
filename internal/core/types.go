package core

import (
	"time"
)

// MachineState represents the current state of any production machine
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

// ProductionOrder represents a manufacturing order (shared across all machine types)
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

// Shift represents a work shift (shared across all machine types)
type Shift struct {
	ShiftID       string         `json:"shiftId"`
	ShiftName     string         `json:"shiftName"`
	ShiftNumber   int            `json:"shiftNumber"`
	StartTime     time.Time      `json:"startTime"`
	EndTime       time.Time      `json:"endTime"`
	WorkCenterID  string         `json:"workCenterId"`
	PlannedBreaks []PlannedBreak `json:"plannedBreaks"`
	Status        string         `json:"status"`
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

// Part represents a physical part flowing through a production line
type Part struct {
	ID       string     `json:"id"`       // Unique part ID, e.g., "PART-2026-01-12-001"
	OrderID  string     `json:"orderId"`  // Associated production order
	Status   PartStatus `json:"status"`   // Current part status
	Location string     `json:"location"` // Current machine/buffer location

	// Timestamps for each stage
	CreatedAt         time.Time `json:"createdAt"`
	FormingComplete   time.Time `json:"formingComplete,omitempty"`
	PickingComplete   time.Time `json:"pickingComplete,omitempty"`
	WeldingComplete   time.Time `json:"weldingComplete,omitempty"`

	// Genealogy - which machines processed this part
	FormingMachineID string `json:"formingMachineId,omitempty"`
	PickerRobotID    string `json:"pickerRobotId,omitempty"`
	SpotWelderID     string `json:"spotWelderId,omitempty"`

	// Quality
	IsScrap     bool   `json:"isScrap"`
	ScrapReason string `json:"scrapReason,omitempty"`
}

// PartStatus represents the current status of a part in the production line
type PartStatus string

const (
	PartStatusInForming       PartStatus = "IN_FORMING"
	PartStatusAwaitingPickup  PartStatus = "AWAITING_PICKUP"
	PartStatusInTransit       PartStatus = "IN_TRANSIT"
	PartStatusAwaitingWelding PartStatus = "AWAITING_WELDING"
	PartStatusBeingWelded     PartStatus = "BEING_WELDED"
	PartStatusComplete        PartStatus = "COMPLETE"
	PartStatusScrap           PartStatus = "SCRAP"
)

// PartBuffer represents a buffer between machines
type PartBuffer struct {
	MaxCapacity int     `json:"maxCapacity"`
	Queue       []*Part `json:"queue"`
}

// NewPartBuffer creates a new part buffer with the given capacity
func NewPartBuffer(capacity int) *PartBuffer {
	return &PartBuffer{
		MaxCapacity: capacity,
		Queue:       make([]*Part, 0, capacity),
	}
}

// Push adds a part to the buffer (returns false if full)
func (pb *PartBuffer) Push(part *Part) bool {
	if len(pb.Queue) >= pb.MaxCapacity {
		return false
	}
	pb.Queue = append(pb.Queue, part)
	return true
}

// Pop removes and returns the first part from the buffer (FIFO)
func (pb *PartBuffer) Pop() *Part {
	if len(pb.Queue) == 0 {
		return nil
	}
	part := pb.Queue[0]
	pb.Queue = pb.Queue[1:]
	return part
}

// Peek returns the first part without removing it
func (pb *PartBuffer) Peek() *Part {
	if len(pb.Queue) == 0 {
		return nil
	}
	return pb.Queue[0]
}

// Count returns the number of parts in the buffer
func (pb *PartBuffer) Count() int {
	return len(pb.Queue)
}

// IsFull returns true if the buffer is at capacity
func (pb *PartBuffer) IsFull() bool {
	return len(pb.Queue) >= pb.MaxCapacity
}

// IsEmpty returns true if the buffer has no parts
func (pb *PartBuffer) IsEmpty() bool {
	return len(pb.Queue) == 0
}

// ErrorInfo contains information about a machine error (generic)
type ErrorInfo struct {
	Code        string
	Message     string
	OccurredAt  time.Time
	ExpectedEnd time.Time
}

// LineMetrics holds production line level metrics
type LineMetrics struct {
	LineState            string  `json:"lineState"`            // Running, Stopped, Error
	WIPCount             int     `json:"wipCount"`             // Work in progress count
	ThroughputPerHour    float64 `json:"throughputPerHour"`    // Parts per hour
	BottleneckMachine    string  `json:"bottleneckMachine"`    // Which machine is slowest
	TotalPartsCompleted  int     `json:"totalPartsCompleted"`  // Total parts through line

	// OEE components
	Availability float64 `json:"availability"` // Uptime / total time
	Performance  float64 `json:"performance"`  // Actual / theoretical throughput
	Quality      float64 `json:"quality"`      // Good parts / all parts
	OEE          float64 `json:"oee"`          // Availability * Performance * Quality
}
