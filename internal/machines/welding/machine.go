package welding

import (
	"context"
	"time"

	"github.com/sebastiankruger/shopfloor-simulator/internal/core"
)

// WeldingRobot implements MachineSimulator for a welding robot
type WeldingRobot struct {
	*core.BaseMachine

	// Welding-specific configuration
	weldConfig WeldingConfig

	// Welding-specific state
	weldState WeldingState

	// Noise generator
	noise *core.NoiseGenerator

	// Timeseries generator
	tsGen *TimeseriesGenerator

	// Machine name
	name string
}

// NewWeldingRobot creates a new welding robot simulator
func NewWeldingRobot(name string, cfg core.MachineConfig, weldCfg WeldingConfig) *WeldingRobot {
	wr := &WeldingRobot{
		BaseMachine: core.NewBaseMachine(cfg),
		weldConfig:  weldCfg,
		weldState: WeldingState{
			WeldPhase: PhaseOff,
		},
		noise: core.NewNoiseGenerator(),
		name:  name,
	}

	// Create timeseries generator with welding config
	wr.tsGen = NewTimeseriesGenerator(weldCfg)

	return wr
}

// Name returns the machine name
func (wr *WeldingRobot) Name() string {
	return wr.name
}

// MachineType returns "welding"
func (wr *WeldingRobot) MachineType() string {
	return "welding"
}

// Start starts the welding robot
func (wr *WeldingRobot) Start(ctx context.Context) error {
	// Initialize state
	wr.weldState.WeldPhase = PhaseOff
	wr.weldState.ArcTime = 0
	return nil
}

// Stop stops the welding robot
func (wr *WeldingRobot) Stop(ctx context.Context) error {
	return nil
}

// WeldPhase returns the current weld phase
func (wr *WeldingRobot) WeldPhase() WeldPhase {
	return wr.weldState.WeldPhase
}

// SetWeldPhase sets the current weld phase
func (wr *WeldingRobot) SetWeldPhase(phase WeldPhase) {
	wr.weldState.WeldPhase = phase
	wr.weldState.PhaseStartedAt = time.Now()
}

// GetArcTime returns cumulative arc time
func (wr *WeldingRobot) GetArcTime() float64 {
	return wr.weldState.ArcTime
}

// Update is called every tick to update the state machine
func (wr *WeldingRobot) Update(now time.Time, isBreakTime bool) {
	elapsed := wr.ElapsedInState()

	switch wr.State() {
	case core.StateIdle:
		wr.updateIdle(now)

	case core.StateSetup:
		wr.updateSetup(elapsed, now)

	case core.StateRunning:
		wr.updateRunning(now, isBreakTime)

	case core.StatePlannedStop:
		wr.updatePlannedStop(isBreakTime)

	case core.StateUnplannedStop:
		wr.updateUnplannedStop(now)
	}
}

func (wr *WeldingRobot) updateIdle(now time.Time) {
	// Reset weld phase when idle
	wr.weldState.WeldPhase = PhaseOff

	// Check if there's an order to work on
	if wr.CurrentOrder != nil || len(wr.OrderQueue) > 0 {
		if wr.CurrentOrder == nil {
			wr.StartNextOrder(now)
		}
		wr.TransitionTo(core.StateSetup)
	}
}

func (wr *WeldingRobot) updateSetup(elapsed time.Duration, now time.Time) {
	// Setup complete after configured time
	if elapsed >= wr.Config().GetEffectiveSetupTime() {
		wr.TransitionTo(core.StateRunning)
		wr.CycleStartedAt = now
		wr.SetWeldPhase(PhaseRampUp)
	}
}

func (wr *WeldingRobot) updateRunning(now time.Time, isBreakTime bool) {
	// Check for break time
	if isBreakTime {
		wr.TransitionTo(core.StatePlannedStop)
		wr.weldState.WeldPhase = PhaseOff
		return
	}

	// Check for random error
	if wr.shouldTriggerError() {
		wr.triggerError(now)
		return
	}

	// Update weld phase and check cycle completion
	cycleElapsed := wr.ElapsedInCycle()
	cycleTime := wr.Config().GetEffectiveCycleTime()

	// Calculate phase timing
	rampUpDuration := time.Duration(float64(cycleTime) * 0.05)   // 5% of cycle
	rampDownDuration := time.Duration(float64(cycleTime) * 0.05) // 5% of cycle
	steadyDuration := cycleTime - rampUpDuration - rampDownDuration

	switch wr.weldState.WeldPhase {
	case PhaseRampUp:
		if cycleElapsed >= rampUpDuration {
			wr.SetWeldPhase(PhaseSteady)
		}

	case PhaseSteady:
		// Accumulate arc time
		wr.weldState.ArcTime += wr.Config().PublishInterval.Seconds()

		if cycleElapsed >= rampUpDuration+steadyDuration {
			wr.SetWeldPhase(PhaseRampDown)
		}

	case PhaseRampDown:
		if cycleElapsed >= cycleTime {
			wr.completeCycle(now)
		}
	}
}

func (wr *WeldingRobot) updatePlannedStop(isBreakTime bool) {
	// Return to idle when break is over
	if !isBreakTime {
		wr.TransitionTo(core.StateIdle)
	}
}

func (wr *WeldingRobot) updateUnplannedStop(now time.Time) {
	// Check if error duration has passed
	if wr.IsErrorResolved(now) {
		wr.ClearError()
		wr.TransitionTo(core.StateIdle)
	}
}

func (wr *WeldingRobot) shouldTriggerError() bool {
	// Only trigger errors during steady state
	if wr.weldState.WeldPhase != PhaseSteady {
		return false
	}
	return wr.noise.ShouldTrigger(wr.Config().GetEffectiveErrorRate(), wr.Config().PublishInterval, wr.Config().GetEffectiveCycleTime())
}

func (wr *WeldingRobot) triggerError(now time.Time) {
	// Select random error type
	errors := AllErrorCodes()
	errorCode := errors[wr.noise.UniformInt(0, len(errors)-1)]
	message, minDur, maxDur := GetErrorInfo(errorCode)

	// Random duration within range
	duration := time.Duration(wr.noise.Uniform(float64(minDur), float64(maxDur)))

	wr.TriggerError(string(errorCode), message, duration)
	wr.weldState.WeldPhase = PhaseOff
}

func (wr *WeldingRobot) completeCycle(now time.Time) {
	// Determine if part is scrap
	isScrap := wr.noise.Bool(wr.Config().GetEffectiveScrapRate())

	wr.CompleteCycle(isScrap)

	// Check if order is complete
	if wr.IsOrderComplete() {
		wr.FinishOrder()
		wr.TransitionTo(core.StateIdle)
		wr.weldState.WeldPhase = PhaseOff
		return
	}

	// Start next cycle
	wr.CycleStartedAt = now
	wr.SetWeldPhase(PhaseRampUp)
}

// GetCycleProgress returns the current cycle progress (0-100%)
func (wr *WeldingRobot) GetCycleProgress() float64 {
	if wr.State() != core.StateRunning {
		return 0
	}
	elapsed := wr.ElapsedInCycle()
	progress := float64(elapsed) / float64(wr.Config().GetEffectiveCycleTime()) * 100
	if progress > 100 {
		progress = 100
	}
	return progress
}

// GetOPCUANodes returns the OPC UA node definitions for a welding robot
func (wr *WeldingRobot) GetOPCUANodes() []core.NodeDefinition {
	return []core.NodeDefinition{
		{Name: "WeldingCurrent", DisplayName: "Welding Current", Description: "Current in Amps", DataType: core.DataTypeDouble, Unit: "A", InitialValue: 0.0},
		{Name: "Voltage", DisplayName: "Voltage", Description: "Arc voltage in Volts", DataType: core.DataTypeDouble, Unit: "V", InitialValue: 0.0},
		{Name: "WireFeedSpeed", DisplayName: "Wire Feed Speed", Description: "Wire feed in m/min", DataType: core.DataTypeDouble, Unit: "m/min", InitialValue: 0.0},
		{Name: "GasFlow", DisplayName: "Gas Flow", Description: "Shielding gas flow l/min", DataType: core.DataTypeDouble, Unit: "l/min", InitialValue: 0.0},
		{Name: "TravelSpeed", DisplayName: "Travel Speed", Description: "Travel speed mm/s", DataType: core.DataTypeDouble, Unit: "mm/s", InitialValue: 0.0},
		{Name: "ArcTime", DisplayName: "Arc Time", Description: "Cumulative arc time seconds", DataType: core.DataTypeDouble, Unit: "s", InitialValue: 0.0},
		{Name: "Position.X", DisplayName: "Position X", Description: "X position mm", DataType: core.DataTypeDouble, Unit: "mm", InitialValue: 0.0},
		{Name: "Position.Y", DisplayName: "Position Y", Description: "Y position mm", DataType: core.DataTypeDouble, Unit: "mm", InitialValue: 0.0},
		{Name: "Position.Z", DisplayName: "Position Z", Description: "Z position mm", DataType: core.DataTypeDouble, Unit: "mm", InitialValue: 200.0},
		{Name: "TorchAngle", DisplayName: "Torch Angle", Description: "Torch angle degrees", DataType: core.DataTypeDouble, Unit: "deg", InitialValue: 0.0},
		{Name: "State", DisplayName: "State", Description: "Machine state (0-4)", DataType: core.DataTypeInt32, Unit: "", InitialValue: int32(0)},
		{Name: "GoodParts", DisplayName: "Good Parts", Description: "Good parts count", DataType: core.DataTypeInt32, Unit: "", InitialValue: int32(0)},
		{Name: "ScrapParts", DisplayName: "Scrap Parts", Description: "Scrap parts count", DataType: core.DataTypeInt32, Unit: "", InitialValue: int32(0)},
		{Name: "CurrentOrderId", DisplayName: "Current Order ID", Description: "Active order ID", DataType: core.DataTypeString, Unit: "", InitialValue: ""},
		{Name: "CurrentPartNumber", DisplayName: "Current Part Number", Description: "Active part number", DataType: core.DataTypeString, Unit: "", InitialValue: ""},
		{Name: "CycleProgress", DisplayName: "Cycle Progress", Description: "Progress 0-100%", DataType: core.DataTypeDouble, Unit: "%", InitialValue: 0.0},
		{Name: "ErrorCode", DisplayName: "Error Code", Description: "Current error code", DataType: core.DataTypeString, Unit: "", InitialValue: ""},
		{Name: "ErrorMessage", DisplayName: "Error Message", Description: "Error description", DataType: core.DataTypeString, Unit: "", InitialValue: ""},
	}
}

// GenerateData generates current timeseries data
func (wr *WeldingRobot) GenerateData() map[string]interface{} {
	// Calculate phase progress
	phaseProgress := wr.calculatePhaseProgress()

	// Generate welding data
	data := wr.tsGen.Generate(wr.State(), wr.weldState.WeldPhase, phaseProgress)

	// Add production data
	data.State = wr.State()
	data.GoodParts, data.ScrapParts = wr.GetCounters()
	data.CycleProgress = wr.GetCycleProgress()
	data.ArcTime = wr.weldState.ArcTime

	if wr.CurrentOrder != nil {
		data.CurrentOrderID = wr.CurrentOrder.OrderID
		data.CurrentPartNumber = wr.CurrentOrder.PartNumber
	}

	if wr.CurrentError != nil {
		data.ErrorCode = wr.CurrentError.Code
		data.ErrorMessage = wr.CurrentError.Message
		data.ErrorTimestamp = wr.CurrentError.OccurredAt
	}

	return data.ToMap()
}

func (wr *WeldingRobot) calculatePhaseProgress() float64 {
	cycleTime := wr.Config().GetEffectiveCycleTime()
	rampUpDuration := time.Duration(float64(cycleTime) * 0.05)
	steadyDuration := time.Duration(float64(cycleTime) * 0.90)
	rampDownDuration := time.Duration(float64(cycleTime) * 0.05)

	elapsed := wr.ElapsedInCycle()

	switch wr.weldState.WeldPhase {
	case PhaseRampUp:
		return float64(elapsed) / float64(rampUpDuration)

	case PhaseSteady:
		steadyElapsed := elapsed - rampUpDuration
		return float64(steadyElapsed) / float64(steadyDuration)

	case PhaseRampDown:
		rampDownElapsed := elapsed - rampUpDuration - steadyDuration
		return float64(rampDownElapsed) / float64(rampDownDuration)

	default:
		return 0
	}
}

// GetWeldingData returns fully populated welding data struct
func (wr *WeldingRobot) GetWeldingData() *WeldingData {
	phaseProgress := wr.calculatePhaseProgress()
	data := wr.tsGen.Generate(wr.State(), wr.weldState.WeldPhase, phaseProgress)

	// Add production data
	data.State = wr.State()
	data.Phase = wr.weldState.WeldPhase
	data.GoodParts, data.ScrapParts = wr.GetCounters()
	data.CycleProgress = wr.GetCycleProgress()
	data.ArcTime = wr.weldState.ArcTime

	if wr.CurrentOrder != nil {
		data.CurrentOrderID = wr.CurrentOrder.OrderID
		data.CurrentPartNumber = wr.CurrentOrder.PartNumber
	}

	if wr.CurrentError != nil {
		data.ErrorCode = wr.CurrentError.Code
		data.ErrorMessage = wr.CurrentError.Message
		data.ErrorTimestamp = wr.CurrentError.OccurredAt
	}

	return data
}
