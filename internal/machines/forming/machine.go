package forming

import (
	"context"
	"fmt"
	"time"

	"github.com/sebastiankruger/shopfloor-simulator/internal/core"
)

// FormingMachine implements MachineSimulator for a sheet metal forming machine
type FormingMachine struct {
	*core.BaseMachine

	// Forming-specific configuration
	formConfig FormingConfig

	// Forming-specific state
	formState FormingState

	// Noise generator
	noise *core.NoiseGenerator

	// Timeseries generator
	tsGen *TimeseriesGenerator

	// Machine name
	name string

	// Part counter for generating unique IDs
	partCounter int
}

// NewFormingMachine creates a new forming machine simulator
func NewFormingMachine(name string, cfg core.MachineConfig, formCfg FormingConfig) *FormingMachine {
	fm := &FormingMachine{
		BaseMachine: core.NewBaseMachine(cfg),
		formConfig:  formCfg,
		formState: FormingState{
			Phase:              PhaseIdle,
			CurrentRamPosition: 0, // Start at top
			OutputBuffer:       core.NewPartBuffer(formCfg.OutputBufferCapacity),
		},
		noise: core.NewNoiseGenerator(),
		name:  name,
	}

	// Create timeseries generator with forming config
	fm.tsGen = NewTimeseriesGenerator(formCfg)

	return fm
}

// Name returns the machine name
func (fm *FormingMachine) Name() string {
	return fm.name
}

// MachineType returns "forming"
func (fm *FormingMachine) MachineType() string {
	return "forming"
}

// Start starts the forming machine
func (fm *FormingMachine) Start(ctx context.Context) error {
	fm.formState.Phase = PhaseIdle
	fm.formState.CycleCount = 0
	fm.formState.CurrentRamPosition = 0
	return nil
}

// Stop stops the forming machine
func (fm *FormingMachine) Stop(ctx context.Context) error {
	return nil
}

// Phase returns the current forming phase
func (fm *FormingMachine) Phase() FormingPhase {
	return fm.formState.Phase
}

// SetPhase sets the current forming phase
func (fm *FormingMachine) SetPhase(phase FormingPhase) {
	fm.formState.Phase = phase
	fm.formState.PhaseStartedAt = time.Now()
}

// GetCycleCount returns the cycle count
func (fm *FormingMachine) GetCycleCount() int {
	return fm.formState.CycleCount
}

// GetOutputBuffer returns the output buffer
func (fm *FormingMachine) GetOutputBuffer() *core.PartBuffer {
	return fm.formState.OutputBuffer
}

// Update is called every tick to update the state machine
func (fm *FormingMachine) Update(now time.Time, isBreakTime bool) {
	elapsed := fm.ElapsedInState()

	switch fm.State() {
	case core.StateIdle:
		fm.updateIdle(now)

	case core.StateSetup:
		fm.updateSetup(elapsed, now)

	case core.StateRunning:
		fm.updateRunning(now, isBreakTime)

	case core.StatePlannedStop:
		fm.updatePlannedStop(isBreakTime)

	case core.StateUnplannedStop:
		fm.updateUnplannedStop(now)
	}
}

func (fm *FormingMachine) updateIdle(now time.Time) {
	fm.formState.Phase = PhaseIdle
	fm.formState.CurrentRamPosition = 0 // Ram at top

	// Debug: Log order queue status every few seconds
	if now.Second()%5 == 0 {
		fmt.Printf("[DEBUG] FormingMachine.updateIdle: CurrentOrder=%v, OrderQueue len=%d\n", fm.CurrentOrder != nil, len(fm.OrderQueue))
	}

	// Check if there's an order to work on
	if fm.CurrentOrder != nil || len(fm.OrderQueue) > 0 {
		if fm.CurrentOrder == nil {
			fm.StartNextOrder(now)
		}
		fmt.Printf("[DEBUG] FormingMachine: Starting order, transitioning to Setup\n")
		fm.TransitionTo(core.StateSetup)
	}
}

func (fm *FormingMachine) updateSetup(elapsed time.Duration, now time.Time) {
	// Setup: heating dies, calibration, etc.
	if elapsed >= fm.Config().SetupTime {
		fm.TransitionTo(core.StateRunning)
		fm.CycleStartedAt = now
		fm.SetPhase(PhaseLoad)
		fm.generatePartID(now)
	}
}

func (fm *FormingMachine) updateRunning(now time.Time, isBreakTime bool) {
	// Check for break time
	if isBreakTime {
		fm.TransitionTo(core.StatePlannedStop)
		fm.formState.Phase = PhaseIdle
		return
	}

	// Check for random error
	if fm.shouldTriggerError() {
		fm.triggerError(now)
		return
	}

	// Check if output buffer is full - need to pause
	if fm.formState.OutputBuffer.IsFull() && fm.formState.Phase == PhaseEject {
		// Wait for buffer to clear
		return
	}

	// Update forming phase
	cycleElapsed := fm.ElapsedInCycle()
	cycleTime := fm.Config().CycleTime

	// Calculate phase boundaries
	loadEnd := time.Duration(float64(cycleTime) * fm.formConfig.LoadFraction)
	formEnd := loadEnd + time.Duration(float64(cycleTime)*fm.formConfig.FormFraction)
	holdEnd := formEnd + time.Duration(float64(cycleTime)*fm.formConfig.HoldFraction)
	ejectEnd := holdEnd + time.Duration(float64(cycleTime)*fm.formConfig.EjectFraction)
	// raiseEnd = cycleTime

	switch fm.formState.Phase {
	case PhaseLoad:
		// Ram at top, loading sheet metal
		fm.formState.CurrentRamPosition = 0
		if cycleElapsed >= loadEnd {
			fm.SetPhase(PhaseForm)
		}

	case PhaseForm:
		// Ram descending
		phaseProgress := float64(cycleElapsed-loadEnd) / float64(formEnd-loadEnd)
		fm.formState.CurrentRamPosition = phaseProgress * fm.formConfig.RamTravel
		if cycleElapsed >= formEnd {
			fm.SetPhase(PhaseHold)
		}

	case PhaseHold:
		// Ram at bottom, holding pressure
		fm.formState.CurrentRamPosition = fm.formConfig.RamTravel
		if cycleElapsed >= holdEnd {
			fm.SetPhase(PhaseEject)
		}

	case PhaseEject:
		// Part ejection
		if cycleElapsed >= ejectEnd {
			fm.ejectPart(now)
			fm.SetPhase(PhaseRaise)
		}

	case PhaseRaise:
		// Ram returning to top
		phaseProgress := float64(cycleElapsed-ejectEnd) / float64(cycleTime-ejectEnd)
		fm.formState.CurrentRamPosition = fm.formConfig.RamTravel * (1 - phaseProgress)
		if cycleElapsed >= cycleTime {
			fm.completeCycle(now)
		}
	}
}

func (fm *FormingMachine) updatePlannedStop(isBreakTime bool) {
	if !isBreakTime {
		fm.TransitionTo(core.StateIdle)
	}
}

func (fm *FormingMachine) updateUnplannedStop(now time.Time) {
	if fm.IsErrorResolved(now) {
		fm.ClearError()
		fm.TransitionTo(core.StateIdle)
	}
}

func (fm *FormingMachine) shouldTriggerError() bool {
	// Only trigger errors during form or hold phase
	if fm.formState.Phase != PhaseForm && fm.formState.Phase != PhaseHold {
		return false
	}
	return fm.noise.ShouldTrigger(fm.Config().ErrorRate, fm.Config().PublishInterval, fm.Config().CycleTime)
}

func (fm *FormingMachine) triggerError(now time.Time) {
	errors := AllErrorCodes()
	errorCode := errors[fm.noise.UniformInt(0, len(errors)-1)]
	message, minDur, maxDur := GetErrorInfo(errorCode)
	duration := time.Duration(fm.noise.Uniform(float64(minDur), float64(maxDur)))

	fm.TriggerError(string(errorCode), message, duration)
	fm.formState.Phase = PhaseIdle
}

func (fm *FormingMachine) generatePartID(now time.Time) {
	fm.partCounter++
	fm.formState.CurrentPartID = fmt.Sprintf("PART-%s-%04d",
		now.Format("2006-01-02"),
		fm.partCounter)
}

func (fm *FormingMachine) ejectPart(now time.Time) {
	// Determine if part is scrap
	isScrap := fm.noise.Bool(fm.Config().ScrapRate)

	if isScrap {
		fm.ScrapParts++
		if fm.CurrentOrder != nil {
			fm.CurrentOrder.QuantityScrap++
		}
	} else {
		// Good part - add to output buffer
		part := &core.Part{
			ID:               fm.formState.CurrentPartID,
			OrderID:          "",
			Status:           core.PartStatusAwaitingPickup,
			Location:         fm.name,
			CreatedAt:        now,
			FormingComplete:  now,
			FormingMachineID: fm.name,
		}
		if fm.CurrentOrder != nil {
			part.OrderID = fm.CurrentOrder.OrderID
		}

		if !fm.formState.OutputBuffer.Push(part) {
			// Buffer full - shouldn't happen if we check before eject
			isScrap = true
			fm.ScrapParts++
		} else {
			fm.GoodParts++
			if fm.CurrentOrder != nil {
				fm.CurrentOrder.QuantityCompleted++
			}
		}
	}

	// Callback for cycle completion
	if fm.BaseMachine != nil {
		fm.CompleteCycle(isScrap)
	}
}

func (fm *FormingMachine) completeCycle(now time.Time) {
	fm.formState.CycleCount++

	// Check if order is complete
	if fm.IsOrderComplete() {
		fm.FinishOrder()
		fm.TransitionTo(core.StateIdle)
		fm.formState.Phase = PhaseIdle
		return
	}

	// Start next cycle
	fm.CycleStartedAt = now
	fm.SetPhase(PhaseLoad)
	fm.generatePartID(now)
}

// GetCycleProgress returns the current cycle progress (0-100%)
func (fm *FormingMachine) GetCycleProgress() float64 {
	if fm.State() != core.StateRunning {
		return 0
	}
	elapsed := fm.ElapsedInCycle()
	progress := float64(elapsed) / float64(fm.Config().CycleTime) * 100
	if progress > 100 {
		progress = 100
	}
	return progress
}

// GetOPCUANodes returns the OPC UA node definitions for a forming machine
func (fm *FormingMachine) GetOPCUANodes() []core.NodeDefinition {
	return []core.NodeDefinition{
		{Name: "Temperature", DisplayName: "Temperature", Description: "Process temperature", DataType: core.DataTypeDouble, Unit: "°C", InitialValue: 25.0},
		{Name: "Pressure", DisplayName: "Pressure", Description: "Hydraulic pressure", DataType: core.DataTypeDouble, Unit: "bar", InitialValue: 0.0},
		{Name: "FormingForce", DisplayName: "Forming Force", Description: "Forming force", DataType: core.DataTypeDouble, Unit: "kN", InitialValue: 0.0},
		{Name: "RamPosition", DisplayName: "Ram Position", Description: "Ram position (0=top)", DataType: core.DataTypeDouble, Unit: "mm", InitialValue: 0.0},
		{Name: "RamSpeed", DisplayName: "Ram Speed", Description: "Ram speed", DataType: core.DataTypeDouble, Unit: "mm/s", InitialValue: 0.0},
		{Name: "DieTemperature", DisplayName: "Die Temperature", Description: "Die surface temperature", DataType: core.DataTypeDouble, Unit: "°C", InitialValue: 25.0},
		{Name: "CycleCount", DisplayName: "Cycle Count", Description: "Total cycles completed", DataType: core.DataTypeInt32, Unit: "", InitialValue: int32(0)},
		{Name: "CycleTime", DisplayName: "Cycle Time", Description: "Current cycle time", DataType: core.DataTypeDouble, Unit: "s", InitialValue: 0.0},
		{Name: "State", DisplayName: "State", Description: "Machine state (0-4)", DataType: core.DataTypeInt32, Unit: "", InitialValue: int32(0)},
		{Name: "GoodParts", DisplayName: "Good Parts", Description: "Good parts count", DataType: core.DataTypeInt32, Unit: "", InitialValue: int32(0)},
		{Name: "ScrapParts", DisplayName: "Scrap Parts", Description: "Scrap parts count", DataType: core.DataTypeInt32, Unit: "", InitialValue: int32(0)},
		{Name: "CurrentOrderId", DisplayName: "Current Order ID", Description: "Active order ID", DataType: core.DataTypeString, Unit: "", InitialValue: ""},
		{Name: "CurrentPartNumber", DisplayName: "Current Part Number", Description: "Active part number", DataType: core.DataTypeString, Unit: "", InitialValue: ""},
		{Name: "CycleProgress", DisplayName: "Cycle Progress", Description: "Progress 0-100%", DataType: core.DataTypeDouble, Unit: "%", InitialValue: 0.0},
		{Name: "OutputBufferCount", DisplayName: "Output Buffer Count", Description: "Parts in output buffer", DataType: core.DataTypeInt32, Unit: "", InitialValue: int32(0)},
		{Name: "CurrentPartId", DisplayName: "Current Part ID", Description: "Current part being formed", DataType: core.DataTypeString, Unit: "", InitialValue: ""},
		{Name: "ErrorCode", DisplayName: "Error Code", Description: "Current error code", DataType: core.DataTypeString, Unit: "", InitialValue: ""},
		{Name: "ErrorMessage", DisplayName: "Error Message", Description: "Error description", DataType: core.DataTypeString, Unit: "", InitialValue: ""},
	}
}

// GenerateData generates current timeseries data
func (fm *FormingMachine) GenerateData() map[string]interface{} {
	// Calculate phase progress
	phaseProgress := fm.calculatePhaseProgress()

	// Generate forming data
	data := fm.tsGen.Generate(fm.State(), fm.formState.Phase, phaseProgress, fm.formState.CurrentRamPosition)

	// Add production data
	data.State = fm.State()
	data.Phase = fm.formState.Phase
	data.GoodParts, data.ScrapParts = fm.GetCounters()
	data.CycleProgress = fm.GetCycleProgress()
	data.CycleCount = fm.formState.CycleCount
	data.CycleTime = fm.ElapsedInCycle().Seconds()
	data.OutputBufferCount = fm.formState.OutputBuffer.Count()
	data.CurrentPartID = fm.formState.CurrentPartID

	if fm.CurrentOrder != nil {
		data.CurrentOrderID = fm.CurrentOrder.OrderID
		data.CurrentPartNumber = fm.CurrentOrder.PartNumber
	}

	if fm.CurrentError != nil {
		data.ErrorCode = fm.CurrentError.Code
		data.ErrorMessage = fm.CurrentError.Message
		data.ErrorTimestamp = fm.CurrentError.OccurredAt
	}

	return data.ToMap()
}

func (fm *FormingMachine) calculatePhaseProgress() float64 {
	cycleTime := fm.Config().CycleTime
	elapsed := fm.ElapsedInCycle()

	loadEnd := time.Duration(float64(cycleTime) * fm.formConfig.LoadFraction)
	formEnd := loadEnd + time.Duration(float64(cycleTime)*fm.formConfig.FormFraction)
	holdEnd := formEnd + time.Duration(float64(cycleTime)*fm.formConfig.HoldFraction)
	ejectEnd := holdEnd + time.Duration(float64(cycleTime)*fm.formConfig.EjectFraction)

	switch fm.formState.Phase {
	case PhaseLoad:
		return float64(elapsed) / float64(loadEnd)
	case PhaseForm:
		return float64(elapsed-loadEnd) / float64(formEnd-loadEnd)
	case PhaseHold:
		return float64(elapsed-formEnd) / float64(holdEnd-formEnd)
	case PhaseEject:
		return float64(elapsed-holdEnd) / float64(ejectEnd-holdEnd)
	case PhaseRaise:
		return float64(elapsed-ejectEnd) / float64(cycleTime-ejectEnd)
	default:
		return 0
	}
}

// GetFormingData returns fully populated forming data struct
func (fm *FormingMachine) GetFormingData() *FormingData {
	phaseProgress := fm.calculatePhaseProgress()
	data := fm.tsGen.Generate(fm.State(), fm.formState.Phase, phaseProgress, fm.formState.CurrentRamPosition)

	// Add production data
	data.State = fm.State()
	data.Phase = fm.formState.Phase
	data.GoodParts, data.ScrapParts = fm.GetCounters()
	data.CycleProgress = fm.GetCycleProgress()
	data.CycleCount = fm.formState.CycleCount
	data.CycleTime = fm.ElapsedInCycle().Seconds()
	data.OutputBufferCount = fm.formState.OutputBuffer.Count()
	data.CurrentPartID = fm.formState.CurrentPartID

	if fm.CurrentOrder != nil {
		data.CurrentOrderID = fm.CurrentOrder.OrderID
		data.CurrentPartNumber = fm.CurrentOrder.PartNumber
	}

	if fm.CurrentError != nil {
		data.ErrorCode = fm.CurrentError.Code
		data.ErrorMessage = fm.CurrentError.Message
		data.ErrorTimestamp = fm.CurrentError.OccurredAt
	}

	return data
}
