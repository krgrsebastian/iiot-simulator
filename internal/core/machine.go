package core

import (
	"context"
	"time"

	"github.com/sebastiankruger/shopfloor-simulator/internal/config"
)

// MachineSimulator defines the interface that all machine types must implement
type MachineSimulator interface {
	// Identity
	Name() string
	MachineType() string

	// Lifecycle
	Start(ctx context.Context) error
	Stop(ctx context.Context) error

	// State management
	State() MachineState
	Update(now time.Time, isBreakTime bool)

	// Production
	AddOrder(order *ProductionOrder)
	GetCurrentOrder() *ProductionOrder
	GetCycleProgress() float64

	// Counters
	GetCounters() (goodParts, scrapParts int)
	ResetCounters()

	// OPC UA node definitions (machine-specific nodes)
	GetOPCUANodes() []NodeDefinition

	// Timeseries data generation
	GenerateData() map[string]interface{}
}

// NodeDefinition describes an OPC UA node for a machine
type NodeDefinition struct {
	Name        string      // Node name (e.g., "WeldingCurrent")
	DisplayName string      // Human-readable name
	Description string      // Description of the node
	DataType    DataType    // Data type (Double, Int32, String, etc.)
	Unit        string      // Engineering unit (A, V, mm, etc.)
	InitialValue interface{} // Initial/default value
}

// DataType represents OPC UA data types
type DataType int

const (
	DataTypeDouble DataType = iota
	DataTypeFloat
	DataTypeInt32
	DataTypeInt64
	DataTypeString
	DataTypeBool
	DataTypeDateTime
)

// MachineConfig holds configuration common to all machine types
type MachineConfig struct {
	Name            string
	CycleTime       time.Duration
	SetupTime       time.Duration
	ScrapRate       float64
	ErrorRate       float64
	PublishInterval time.Duration
	Runtime         *config.RuntimeConfig // Runtime-adjustable config (optional)
}

// GetEffectiveCycleTime returns the cycle time, using runtime config if available.
func (mc MachineConfig) GetEffectiveCycleTime() time.Duration {
	if mc.Runtime != nil {
		return mc.Runtime.GetEffectiveCycleTime()
	}
	return mc.CycleTime
}

// GetEffectiveSetupTime returns the setup time, using runtime config if available.
func (mc MachineConfig) GetEffectiveSetupTime() time.Duration {
	if mc.Runtime != nil {
		return mc.Runtime.GetEffectiveSetupTime()
	}
	return mc.SetupTime
}

// GetEffectiveScrapRate returns the scrap rate, using runtime config if available.
func (mc MachineConfig) GetEffectiveScrapRate() float64 {
	if mc.Runtime != nil {
		return mc.Runtime.GetScrapRate()
	}
	return mc.ScrapRate
}

// GetEffectiveErrorRate returns the error rate, using runtime config if available.
func (mc MachineConfig) GetEffectiveErrorRate() float64 {
	if mc.Runtime != nil {
		return mc.Runtime.GetErrorRate()
	}
	return mc.ErrorRate
}

// GetEffectiveErrorDuration scales an error duration by the cycle time scale.
// Higher scale = faster simulation = shorter error duration.
func (mc MachineConfig) GetEffectiveErrorDuration(baseDuration time.Duration) time.Duration {
	if mc.Runtime != nil {
		return mc.Runtime.GetEffectiveErrorDuration(baseDuration)
	}
	return baseDuration
}

// Callbacks for machine events
type MachineCallbacks struct {
	OnStateChange   func(from, to MachineState)
	OnCycleComplete func(isScrap bool)
	OnOrderComplete func(order *ProductionOrder)
	OnError         func(err *ErrorInfo)
	OnPartProduced  func(part *Part) // For production line coordination
}

// BaseMachine provides common functionality for all machine types
type BaseMachine struct {
	config    MachineConfig
	callbacks MachineCallbacks
	state     MachineState

	// Timing
	StateEnteredAt time.Time
	CycleStartedAt time.Time

	// Current work
	CurrentOrder *ProductionOrder
	CurrentShift *Shift
	OrderQueue   []*ProductionOrder

	// Counters
	GoodParts  int
	ScrapParts int

	// Error state
	CurrentError *ErrorInfo
}

// NewBaseMachine creates a new base machine with common initialization
func NewBaseMachine(cfg MachineConfig) *BaseMachine {
	return &BaseMachine{
		config:         cfg,
		state:          StateIdle,
		StateEnteredAt: time.Now(),
		OrderQueue:     make([]*ProductionOrder, 0),
	}
}

// SetCallbacks sets the event callbacks
func (bm *BaseMachine) SetCallbacks(cb MachineCallbacks) {
	bm.callbacks = cb
}

// State returns the current machine state
func (bm *BaseMachine) State() MachineState {
	return bm.state
}

// TransitionTo changes the machine state
func (bm *BaseMachine) TransitionTo(newState MachineState) {
	if bm.state == newState {
		return
	}

	oldState := bm.state
	bm.state = newState
	bm.StateEnteredAt = time.Now()

	if bm.callbacks.OnStateChange != nil {
		bm.callbacks.OnStateChange(oldState, newState)
	}
}

// AddOrder adds a production order to the queue
func (bm *BaseMachine) AddOrder(order *ProductionOrder) {
	bm.OrderQueue = append(bm.OrderQueue, order)
}

// GetCurrentOrder returns the current production order
func (bm *BaseMachine) GetCurrentOrder() *ProductionOrder {
	return bm.CurrentOrder
}

// GetCounters returns production counters
func (bm *BaseMachine) GetCounters() (goodParts, scrapParts int) {
	return bm.GoodParts, bm.ScrapParts
}

// ResetCounters resets the shift counters
func (bm *BaseMachine) ResetCounters() {
	bm.GoodParts = 0
	bm.ScrapParts = 0
}

// SetCurrentShift sets the current shift
func (bm *BaseMachine) SetCurrentShift(shift *Shift) {
	bm.CurrentShift = shift
}

// Config returns the machine configuration
func (bm *BaseMachine) Config() MachineConfig {
	return bm.config
}

// TriggerError triggers an error state
func (bm *BaseMachine) TriggerError(code, message string, duration time.Duration) {
	now := time.Now()
	bm.CurrentError = &ErrorInfo{
		Code:        code,
		Message:     message,
		OccurredAt:  now,
		ExpectedEnd: now.Add(duration),
	}
	bm.TransitionTo(StateUnplannedStop)

	if bm.callbacks.OnError != nil {
		bm.callbacks.OnError(bm.CurrentError)
	}
}

// ClearError clears the current error
func (bm *BaseMachine) ClearError() {
	bm.CurrentError = nil
}

// HasError returns true if there's an active error
func (bm *BaseMachine) HasError() bool {
	return bm.CurrentError != nil
}

// IsErrorResolved returns true if the error duration has passed
func (bm *BaseMachine) IsErrorResolved(now time.Time) bool {
	return bm.CurrentError != nil && now.After(bm.CurrentError.ExpectedEnd)
}

// StartNextOrder dequeues and starts the next order
func (bm *BaseMachine) StartNextOrder(now time.Time) bool {
	if len(bm.OrderQueue) == 0 {
		return false
	}

	bm.CurrentOrder = bm.OrderQueue[0]
	bm.OrderQueue = bm.OrderQueue[1:]
	bm.CurrentOrder.Status = OrderStatusInProgress
	bm.CurrentOrder.StartedAt = now
	return true
}

// CompleteCycle handles cycle completion logic
func (bm *BaseMachine) CompleteCycle(isScrap bool) {
	if isScrap {
		bm.ScrapParts++
		if bm.CurrentOrder != nil {
			bm.CurrentOrder.QuantityScrap++
		}
	} else {
		bm.GoodParts++
		if bm.CurrentOrder != nil {
			bm.CurrentOrder.QuantityCompleted++
		}
	}

	if bm.callbacks.OnCycleComplete != nil {
		bm.callbacks.OnCycleComplete(isScrap)
	}
}

// IsOrderComplete returns true if the current order is finished
func (bm *BaseMachine) IsOrderComplete() bool {
	if bm.CurrentOrder == nil {
		return false
	}
	total := bm.CurrentOrder.QuantityCompleted + bm.CurrentOrder.QuantityScrap
	return total >= bm.CurrentOrder.Quantity
}

// FinishOrder marks the current order as complete
func (bm *BaseMachine) FinishOrder() {
	if bm.CurrentOrder != nil {
		bm.CurrentOrder.Status = OrderStatusCompleted
		if bm.callbacks.OnOrderComplete != nil {
			bm.callbacks.OnOrderComplete(bm.CurrentOrder)
		}
		bm.CurrentOrder = nil
	}
}

// ElapsedInState returns time since entering current state
func (bm *BaseMachine) ElapsedInState() time.Duration {
	return time.Since(bm.StateEnteredAt)
}

// ElapsedInCycle returns time since cycle started
func (bm *BaseMachine) ElapsedInCycle() time.Duration {
	return time.Since(bm.CycleStartedAt)
}
