package simulator

import (
	"time"

	"github.com/sebastiankruger/shopfloor-simulator/internal/config"
	"github.com/sebastiankruger/shopfloor-simulator/internal/core"
	"github.com/sebastiankruger/shopfloor-simulator/internal/machines/welding"
)

// StateMachine wraps the welding robot for backward compatibility
type StateMachine struct {
	robot       *welding.WeldingRobot
	cfg         *config.Config
	onStateChange   func(from, to MachineState)
	onCycleComplete func(isScrap bool)
	onOrderComplete func(order *ProductionOrder)
	onError         func(err *ErrorInfo)
}

// NewStateMachine creates a new state machine wrapping a welding robot
func NewStateMachine(cfg *config.Config) *StateMachine {
	// Create core machine config
	machineCfg := core.MachineConfig{
		Name:            cfg.SimulatorName,
		CycleTime:       cfg.CycleTime,
		SetupTime:       cfg.SetupTime,
		ScrapRate:       cfg.ScrapRate,
		ErrorRate:       cfg.ErrorRate,
		PublishInterval: cfg.PublishInterval,
	}

	// Create welding config with defaults
	weldCfg := welding.DefaultWeldingConfig()

	// Create the welding robot
	robot := welding.NewWeldingRobot(cfg.SimulatorName, machineCfg, weldCfg)

	return &StateMachine{
		robot: robot,
		cfg:   cfg,
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

	// Set callbacks on the robot
	sm.robot.SetCallbacks(core.MachineCallbacks{
		OnStateChange:   onStateChange,
		OnCycleComplete: onCycleComplete,
		OnOrderComplete: onOrderComplete,
		OnError:         onError,
	})
}

// State returns the current machine state
func (sm *StateMachine) State() MachineState {
	return sm.robot.State()
}

// WeldPhase returns the current weld phase
func (sm *StateMachine) WeldPhase() WeldPhase {
	return sm.robot.WeldPhase()
}

// GetState returns a copy of the simulator state
func (sm *StateMachine) GetState() SimulatorState {
	goodParts, scrapParts := sm.robot.GetCounters()
	state := SimulatorState{
		State:        sm.robot.State(),
		WeldPhase:    sm.robot.WeldPhase(),
		CurrentOrder: sm.robot.GetCurrentOrder(),
		GoodParts:    goodParts,
		ScrapParts:   scrapParts,
		ArcTime:      sm.robot.GetArcTime(),
	}

	if sm.robot.HasError() {
		state.CurrentError = sm.robot.CurrentError
	}

	return state
}

// TransitionTo changes the machine state
func (sm *StateMachine) TransitionTo(newState MachineState) {
	sm.robot.TransitionTo(newState)
}

// SetWeldPhase sets the current weld phase
func (sm *StateMachine) SetWeldPhase(phase WeldPhase) {
	sm.robot.SetWeldPhase(phase)
}

// Update is called every tick to update the state machine
func (sm *StateMachine) Update(now time.Time, isBreakTime bool) {
	sm.robot.Update(now, isBreakTime)
}

// AddOrder adds a production order to the queue
func (sm *StateMachine) AddOrder(order *ProductionOrder) {
	sm.robot.AddOrder(order)
}

// SetCurrentShift sets the current shift
func (sm *StateMachine) SetCurrentShift(shift *Shift) {
	sm.robot.SetCurrentShift(shift)
}

// ResetCounters resets the shift counters (called at shift start)
func (sm *StateMachine) ResetCounters() {
	sm.robot.ResetCounters()
}

// GetCounters returns current production counters
func (sm *StateMachine) GetCounters() (goodParts, scrapParts int, arcTime float64) {
	good, scrap := sm.robot.GetCounters()
	return good, scrap, sm.robot.GetArcTime()
}

// GetCurrentOrder returns the current production order
func (sm *StateMachine) GetCurrentOrder() *ProductionOrder {
	return sm.robot.GetCurrentOrder()
}

// GetCycleProgress returns the current cycle progress (0-100%)
func (sm *StateMachine) GetCycleProgress() float64 {
	return sm.robot.GetCycleProgress()
}

// GetWeldingRobot returns the underlying welding robot (for new code)
func (sm *StateMachine) GetWeldingRobot() *welding.WeldingRobot {
	return sm.robot
}
