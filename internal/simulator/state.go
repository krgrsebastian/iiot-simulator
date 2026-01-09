package simulator

import (
	"math/rand"
	"time"

	"github.com/sebastiankruger/shopfloor-simulator/internal/config"
)

// StateMachine handles state transitions for the welding robot
type StateMachine struct {
	state           *SimulatorState
	cfg             *config.Config
	rng             *rand.Rand
	onStateChange   func(from, to MachineState)
	onCycleComplete func(isScrap bool)
	onOrderComplete func(order *ProductionOrder)
	onError         func(err *ErrorInfo)
}

// NewStateMachine creates a new state machine
func NewStateMachine(cfg *config.Config) *StateMachine {
	return &StateMachine{
		state: &SimulatorState{
			State:          StateIdle,
			WeldPhase:      PhaseOff,
			StateEnteredAt: time.Now(),
			OrderQueue:     make([]*ProductionOrder, 0),
		},
		cfg: cfg,
		rng: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// SetCallbacks sets the callback functions for state events
func (sm *StateMachine) SetCallbacks(
	onStateChange func(from, to MachineState),
	onCycleComplete func(isScrap bool),
	onOrderComplete func(order *ProductionOrder),
	onError func(err *ErrorInfo),
) {
	sm.onStateChange = onStateChange
	sm.onCycleComplete = onCycleComplete
	sm.onOrderComplete = onOrderComplete
	sm.onError = onError
}

// State returns the current machine state
func (sm *StateMachine) State() MachineState {
	return sm.state.State
}

// WeldPhase returns the current weld phase
func (sm *StateMachine) WeldPhase() WeldPhase {
	return sm.state.WeldPhase
}

// GetState returns a copy of the simulator state
func (sm *StateMachine) GetState() SimulatorState {
	return *sm.state
}

// TransitionTo changes the machine state
func (sm *StateMachine) TransitionTo(newState MachineState) {
	if sm.state.State == newState {
		return
	}

	oldState := sm.state.State
	sm.state.State = newState
	sm.state.StateEnteredAt = time.Now()

	// Reset weld phase when not running
	if newState != StateRunning {
		sm.state.WeldPhase = PhaseOff
	}

	if sm.onStateChange != nil {
		sm.onStateChange(oldState, newState)
	}
}

// SetWeldPhase sets the current weld phase
func (sm *StateMachine) SetWeldPhase(phase WeldPhase) {
	sm.state.WeldPhase = phase
	sm.state.PhaseStartedAt = time.Now()
}

// Update is called every tick to update the state machine
func (sm *StateMachine) Update(now time.Time, isBreakTime bool) {
	elapsed := now.Sub(sm.state.StateEnteredAt)

	switch sm.state.State {
	case StateIdle:
		sm.updateIdle(now)

	case StateSetup:
		sm.updateSetup(elapsed, now)

	case StateRunning:
		sm.updateRunning(now, isBreakTime)

	case StatePlannedStop:
		sm.updatePlannedStop(isBreakTime)

	case StateUnplannedStop:
		sm.updateUnplannedStop(now)
	}
}

func (sm *StateMachine) updateIdle(now time.Time) {
	// Check if there's an order to work on
	if sm.state.CurrentOrder != nil || len(sm.state.OrderQueue) > 0 {
		if sm.state.CurrentOrder == nil {
			sm.state.CurrentOrder = sm.state.OrderQueue[0]
			sm.state.OrderQueue = sm.state.OrderQueue[1:]
			sm.state.CurrentOrder.Status = OrderStatusInProgress
			sm.state.CurrentOrder.StartedAt = now
		}
		sm.TransitionTo(StateSetup)
	}
}

func (sm *StateMachine) updateSetup(elapsed time.Duration, now time.Time) {
	// Setup complete after configured time
	if elapsed >= sm.cfg.SetupTime {
		sm.TransitionTo(StateRunning)
		sm.state.CycleStartedAt = now
		sm.SetWeldPhase(PhaseRampUp)
	}
}

func (sm *StateMachine) updateRunning(now time.Time, isBreakTime bool) {
	// Check for break time
	if isBreakTime {
		sm.TransitionTo(StatePlannedStop)
		return
	}

	// Check for random error
	if sm.shouldTriggerError() {
		sm.triggerError(now)
		return
	}

	// Update weld phase and check cycle completion
	cycleElapsed := now.Sub(sm.state.CycleStartedAt)
	cycleTime := sm.cfg.CycleTime

	// Get part-specific cycle time if available
	if sm.state.CurrentOrder != nil {
		// Could look up part-specific cycle time here
	}

	// Calculate phase timing
	rampUpDuration := time.Duration(float64(cycleTime) * 0.05)   // 5% of cycle
	rampDownDuration := time.Duration(float64(cycleTime) * 0.05) // 5% of cycle
	steadyDuration := cycleTime - rampUpDuration - rampDownDuration

	switch sm.state.WeldPhase {
	case PhaseRampUp:
		if cycleElapsed >= rampUpDuration {
			sm.SetWeldPhase(PhaseSteady)
		}

	case PhaseSteady:
		// Accumulate arc time
		sm.state.ArcTime += sm.cfg.PublishInterval.Seconds()

		if cycleElapsed >= rampUpDuration+steadyDuration {
			sm.SetWeldPhase(PhaseRampDown)
		}

	case PhaseRampDown:
		if cycleElapsed >= cycleTime {
			sm.completeCycle(now)
		}
	}
}

func (sm *StateMachine) updatePlannedStop(isBreakTime bool) {
	// Return to idle when break is over
	if !isBreakTime {
		sm.TransitionTo(StateIdle)
	}
}

func (sm *StateMachine) updateUnplannedStop(now time.Time) {
	// Check if error duration has passed
	if sm.state.CurrentError != nil && now.After(sm.state.CurrentError.ExpectedEnd) {
		sm.clearError()
		sm.TransitionTo(StateIdle)
	}
}

func (sm *StateMachine) shouldTriggerError() bool {
	// Only trigger errors during steady state
	if sm.state.WeldPhase != PhaseSteady {
		return false
	}
	return sm.rng.Float64() < sm.cfg.ErrorRate/100 // Convert per-cycle to per-tick
}

func (sm *StateMachine) triggerError(now time.Time) {
	// Select random error type
	errors := []ErrorCode{
		ErrorWireFeedJam,
		ErrorGasFlowFault,
		ErrorArcFault,
		ErrorRobotCollision,
		ErrorQualityReject,
	}
	errorCode := errors[sm.rng.Intn(len(errors))]
	message, minDur, maxDur := GetErrorInfo(errorCode)

	// Random duration within range
	duration := minDur + time.Duration(sm.rng.Float64()*float64(maxDur-minDur))

	sm.state.CurrentError = &ErrorInfo{
		Code:        errorCode,
		Message:     message,
		OccurredAt:  now,
		ExpectedEnd: now.Add(duration),
	}

	sm.TransitionTo(StateUnplannedStop)

	if sm.onError != nil {
		sm.onError(sm.state.CurrentError)
	}
}

func (sm *StateMachine) clearError() {
	sm.state.CurrentError = nil
}

func (sm *StateMachine) completeCycle(now time.Time) {
	// Determine if part is scrap
	isScrap := sm.rng.Float64() < sm.cfg.ScrapRate

	if isScrap {
		sm.state.ScrapParts++
		if sm.state.CurrentOrder != nil {
			sm.state.CurrentOrder.QuantityScrap++
		}
	} else {
		sm.state.GoodParts++
		if sm.state.CurrentOrder != nil {
			sm.state.CurrentOrder.QuantityCompleted++
		}
	}

	if sm.onCycleComplete != nil {
		sm.onCycleComplete(isScrap)
	}

	// Check if order is complete
	if sm.state.CurrentOrder != nil {
		total := sm.state.CurrentOrder.QuantityCompleted + sm.state.CurrentOrder.QuantityScrap
		if total >= sm.state.CurrentOrder.Quantity {
			sm.state.CurrentOrder.Status = OrderStatusCompleted
			if sm.onOrderComplete != nil {
				sm.onOrderComplete(sm.state.CurrentOrder)
			}
			sm.state.CurrentOrder = nil
			sm.TransitionTo(StateIdle)
			return
		}
	}

	// Start next cycle
	sm.state.CycleStartedAt = now
	sm.SetWeldPhase(PhaseRampUp)
}

// AddOrder adds a production order to the queue
func (sm *StateMachine) AddOrder(order *ProductionOrder) {
	sm.state.OrderQueue = append(sm.state.OrderQueue, order)
}

// SetCurrentShift sets the current shift
func (sm *StateMachine) SetCurrentShift(shift *Shift) {
	sm.state.CurrentShift = shift
}

// ResetCounters resets the shift counters (called at shift start)
func (sm *StateMachine) ResetCounters() {
	sm.state.GoodParts = 0
	sm.state.ScrapParts = 0
	sm.state.ArcTime = 0
}

// GetCounters returns current production counters
func (sm *StateMachine) GetCounters() (goodParts, scrapParts int, arcTime float64) {
	return sm.state.GoodParts, sm.state.ScrapParts, sm.state.ArcTime
}

// GetCurrentOrder returns the current production order
func (sm *StateMachine) GetCurrentOrder() *ProductionOrder {
	return sm.state.CurrentOrder
}

// GetCycleProgress returns the current cycle progress (0-100%)
func (sm *StateMachine) GetCycleProgress() float64 {
	if sm.state.State != StateRunning {
		return 0
	}
	elapsed := time.Since(sm.state.CycleStartedAt)
	progress := float64(elapsed) / float64(sm.cfg.CycleTime) * 100
	if progress > 100 {
		progress = 100
	}
	return progress
}
