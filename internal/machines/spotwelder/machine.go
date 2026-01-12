package spotwelder

import (
	"context"
	"time"

	"github.com/sebastiankruger/shopfloor-simulator/internal/core"
)

// SpotWelder implements MachineSimulator for a stud spot welding machine
type SpotWelder struct {
	*core.BaseMachine

	// Spot welder-specific configuration
	welderConfig SpotWelderConfig

	// Spot welder-specific state
	welderState SpotWelderState

	// Noise generator
	noise *core.NoiseGenerator

	// Timeseries generator
	tsGen *TimeseriesGenerator

	// Machine name
	name string
}

// NewSpotWelder creates a new spot welder simulator
func NewSpotWelder(name string, cfg core.MachineConfig, welderCfg SpotWelderConfig) *SpotWelder {
	sw := &SpotWelder{
		BaseMachine:  core.NewBaseMachine(cfg),
		welderConfig: welderCfg,
		welderState: SpotWelderState{
			Phase:         PhaseIdle,
			InputBuffer:   core.NewPartBuffer(welderCfg.InputBufferCapacity),
			ElectrodeTemp: 25.0, // Ambient
			PartTemp:      25.0, // Ambient
		},
		noise: core.NewNoiseGenerator(),
		name:  name,
	}

	// Create timeseries generator
	sw.tsGen = NewTimeseriesGenerator(welderCfg)

	return sw
}

// GetInputBuffer returns the input buffer (parts from picker)
func (sw *SpotWelder) GetInputBuffer() *core.PartBuffer {
	return sw.welderState.InputBuffer
}

// Name returns the machine name
func (sw *SpotWelder) Name() string {
	return sw.name
}

// MachineType returns "spotwelder"
func (sw *SpotWelder) MachineType() string {
	return "spotwelder"
}

// Start starts the spot welder
func (sw *SpotWelder) Start(ctx context.Context) error {
	sw.welderState.Phase = PhaseIdle
	sw.welderState.CycleCount = 0
	sw.welderState.ElectrodeTemp = 25.0
	sw.welderState.PartTemp = 25.0
	return nil
}

// Stop stops the spot welder
func (sw *SpotWelder) Stop(ctx context.Context) error {
	return nil
}

// Phase returns the current spot welder phase
func (sw *SpotWelder) Phase() SpotWelderPhase {
	return sw.welderState.Phase
}

// SetPhase sets the current spot welder phase
func (sw *SpotWelder) SetPhase(phase SpotWelderPhase) {
	sw.welderState.Phase = phase
	sw.welderState.PhaseStartedAt = time.Now()
}

// GetCycleCount returns the cycle count
func (sw *SpotWelder) GetCycleCount() int {
	return sw.welderState.CycleCount
}

// GetTotalWelds returns the total weld count
func (sw *SpotWelder) GetTotalWelds() int {
	return sw.welderState.TotalWelds
}

// GetElectrodeWear returns electrode wear percentage (0-100%)
func (sw *SpotWelder) GetElectrodeWear() float64 {
	return float64(sw.welderState.ElectrodeWeldCount) / float64(sw.welderConfig.ElectrodeLifeWelds) * 100
}

// Update is called every tick to update the state machine
func (sw *SpotWelder) Update(now time.Time, isBreakTime bool) {
	elapsed := sw.ElapsedInState()

	// Cool down electrode when not welding
	if sw.welderState.Phase != PhaseWeld {
		sw.coolDown()
	}

	switch sw.State() {
	case core.StateIdle:
		sw.updateIdle(now)

	case core.StateSetup:
		sw.updateSetup(elapsed, now)

	case core.StateRunning:
		sw.updateRunning(now, isBreakTime)

	case core.StatePlannedStop:
		sw.updatePlannedStop(isBreakTime)

	case core.StateUnplannedStop:
		sw.updateUnplannedStop(now)
	}
}

func (sw *SpotWelder) coolDown() {
	ambientTemp := 25.0
	// Electrode cools slowly
	sw.welderState.ElectrodeTemp = ambientTemp + (sw.welderState.ElectrodeTemp-ambientTemp)*0.995
	// Part cools faster
	sw.welderState.PartTemp = ambientTemp + (sw.welderState.PartTemp-ambientTemp)*0.98
}

func (sw *SpotWelder) updateIdle(now time.Time) {
	sw.welderState.Phase = PhaseIdle

	// Check if there's a part to weld
	if sw.welderState.InputBuffer.Count() > 0 {
		sw.TransitionTo(core.StateSetup)
	}
}

func (sw *SpotWelder) updateSetup(elapsed time.Duration, now time.Time) {
	// Setup: electrode check, position calibration
	if elapsed >= sw.Config().SetupTime {
		// Get part from input buffer
		if part := sw.welderState.InputBuffer.Pop(); part != nil {
			sw.welderState.CurrentPartID = part.ID
			sw.welderState.CurrentPart = part
			part.Status = core.PartStatusBeingWelded
			part.Location = sw.name

			sw.TransitionTo(core.StateRunning)
			sw.CycleStartedAt = now
			sw.SetPhase(PhaseLoad)
			sw.welderState.WeldsInCurrentPart = 0
		} else {
			// No part available, go back to idle
			sw.TransitionTo(core.StateIdle)
		}
	}
}

func (sw *SpotWelder) updateRunning(now time.Time, isBreakTime bool) {
	// Check for break time - only stop if not mid-part
	if isBreakTime && sw.welderState.Phase == PhaseIdle {
		sw.TransitionTo(core.StatePlannedStop)
		return
	}

	// Check for random error
	if sw.shouldTriggerError() {
		sw.triggerError(now)
		return
	}

	// Update spot welder phase
	cycleElapsed := sw.ElapsedInCycle()
	cycleTime := sw.Config().CycleTime
	cfg := sw.welderConfig

	// Calculate phase boundaries
	loadEnd := time.Duration(float64(cycleTime) * cfg.LoadFraction)
	clampEnd := loadEnd + time.Duration(float64(cycleTime)*cfg.ClampFraction)
	preWeldEnd := clampEnd + time.Duration(float64(cycleTime)*cfg.PreWeldFraction)
	weldEnd := preWeldEnd + time.Duration(float64(cycleTime)*cfg.WeldFraction)
	holdEnd := weldEnd + time.Duration(float64(cycleTime)*cfg.HoldFraction)
	releaseEnd := holdEnd + time.Duration(float64(cycleTime)*cfg.ReleaseFraction)
	// unloadEnd = cycleTime

	switch sw.welderState.Phase {
	case PhaseLoad:
		if cycleElapsed >= loadEnd {
			sw.SetPhase(PhaseClamp)
		}

	case PhaseClamp:
		if cycleElapsed >= clampEnd {
			sw.SetPhase(PhasePreWeld)
		}

	case PhasePreWeld:
		if cycleElapsed >= preWeldEnd {
			sw.SetPhase(PhaseWeld)
		}

	case PhaseWeld:
		sw.updateWeldPhase(cycleElapsed, preWeldEnd, weldEnd)

	case PhaseHold:
		if cycleElapsed >= holdEnd {
			sw.SetPhase(PhaseRelease)
		}

	case PhaseRelease:
		if cycleElapsed >= releaseEnd {
			sw.SetPhase(PhaseUnload)
		}

	case PhaseUnload:
		if cycleElapsed >= cycleTime {
			sw.completeCycle(now)
		}
	}
}

func (sw *SpotWelder) updateWeldPhase(elapsed, phaseStart, phaseEnd time.Duration) {
	phaseDuration := phaseEnd - phaseStart
	phaseElapsed := elapsed - phaseStart
	phaseProgress := float64(phaseElapsed) / float64(phaseDuration)

	// Calculate which weld we're on
	weldsPerPart := sw.welderConfig.WeldsPerPart
	weldNumber := int(phaseProgress * float64(weldsPerPart))
	if weldNumber >= weldsPerPart {
		weldNumber = weldsPerPart - 1
	}

	// Track welds completed
	if weldNumber > sw.welderState.WeldsInCurrentPart {
		sw.welderState.WeldsInCurrentPart = weldNumber
		sw.welderState.TotalWelds++
		sw.welderState.ElectrodeWeldCount++

		// Temperature rises with each weld
		sw.welderState.ElectrodeTemp += 15.0 // Each weld adds ~15°C
		sw.welderState.PartTemp += 10.0       // Part heats up too
	}

	// Check if all welds complete
	if elapsed >= phaseEnd {
		// Final weld count
		sw.welderState.WeldsInCurrentPart = weldsPerPart
		sw.welderState.TotalWelds++
		sw.welderState.ElectrodeWeldCount++
		sw.SetPhase(PhaseHold)
	}
}

func (sw *SpotWelder) updatePlannedStop(isBreakTime bool) {
	if !isBreakTime {
		sw.TransitionTo(core.StateIdle)
	}
}

func (sw *SpotWelder) updateUnplannedStop(now time.Time) {
	if sw.IsErrorResolved(now) {
		sw.ClearError()
		sw.TransitionTo(core.StateIdle)
	}
}

func (sw *SpotWelder) shouldTriggerError() bool {
	// Only trigger errors during weld phase
	if sw.welderState.Phase != PhaseWeld {
		return false
	}

	// Higher error rate if electrode is worn
	errorRate := sw.Config().ErrorRate
	if sw.GetElectrodeWear() > 80 {
		errorRate *= 3 // Triple error rate when electrode worn
	}

	return sw.noise.ShouldTrigger(errorRate, sw.Config().PublishInterval, sw.Config().CycleTime)
}

func (sw *SpotWelder) triggerError(now time.Time) {
	errors := AllErrorCodes()
	errorCode := errors[sw.noise.UniformInt(0, len(errors)-1)]

	// If electrode is overheated, always trigger overheat error
	if sw.welderState.ElectrodeTemp > sw.welderConfig.MaxElectrodeTemp {
		errorCode = ErrorOverheat
	}

	message, minDur, maxDur := GetErrorInfo(errorCode)
	duration := time.Duration(sw.noise.Uniform(float64(minDur), float64(maxDur)))

	// Quality reject means scrap the current part
	if errorCode == ErrorQualityReject && sw.welderState.CurrentPartID != "" {
		sw.ScrapParts++
		if sw.welderState.CurrentPart != nil {
			sw.welderState.CurrentPart.IsScrap = true
			sw.welderState.CurrentPart.ScrapReason = "Weld quality reject"
			sw.welderState.CurrentPart.Status = core.PartStatusScrap
		}
		sw.welderState.CurrentPartID = ""
		sw.welderState.CurrentPart = nil
	}

	sw.TriggerError(string(errorCode), message, duration)
	sw.welderState.Phase = PhaseIdle
}

func (sw *SpotWelder) completeCycle(now time.Time) {
	sw.welderState.CycleCount++

	// Mark part as complete
	if sw.welderState.CurrentPart != nil {
		sw.welderState.CurrentPart.Status = core.PartStatusComplete
		sw.welderState.CurrentPart.WeldingComplete = now
		sw.welderState.CurrentPart.SpotWelderID = sw.name
	}

	// Determine if part is good or scrap
	isScrap := sw.noise.Bool(sw.Config().ScrapRate)
	if isScrap {
		sw.ScrapParts++
		if sw.welderState.CurrentPart != nil {
			sw.welderState.CurrentPart.IsScrap = true
			sw.welderState.CurrentPart.ScrapReason = "Quality inspection failed"
			sw.welderState.CurrentPart.Status = core.PartStatusScrap
		}
	} else {
		sw.GoodParts++
	}

	sw.CompleteCycle(isScrap)

	// Clear current part
	sw.welderState.CurrentPartID = ""
	sw.welderState.CurrentPart = nil
	sw.welderState.WeldsInCurrentPart = 0

	// Check if more parts to weld
	if sw.welderState.InputBuffer.Count() > 0 {
		// Start next cycle
		sw.CycleStartedAt = now
		if part := sw.welderState.InputBuffer.Pop(); part != nil {
			sw.welderState.CurrentPartID = part.ID
			sw.welderState.CurrentPart = part
			part.Status = core.PartStatusBeingWelded
			part.Location = sw.name
			sw.SetPhase(PhaseLoad)
		}
	} else {
		// Return to idle
		sw.TransitionTo(core.StateIdle)
		sw.welderState.Phase = PhaseIdle
	}
}

// GetCycleProgress returns the current cycle progress (0-100%)
func (sw *SpotWelder) GetCycleProgress() float64 {
	if sw.State() != core.StateRunning {
		return 0
	}
	elapsed := sw.ElapsedInCycle()
	progress := float64(elapsed) / float64(sw.Config().CycleTime) * 100
	if progress > 100 {
		progress = 100
	}
	return progress
}

// GetOPCUANodes returns the OPC UA node definitions for a spot welder
func (sw *SpotWelder) GetOPCUANodes() []core.NodeDefinition {
	return []core.NodeDefinition{
		{Name: "WeldCurrent", DisplayName: "Weld Current", Description: "Welding current", DataType: core.DataTypeDouble, Unit: "kA", InitialValue: 0.0},
		{Name: "WeldVoltage", DisplayName: "Weld Voltage", Description: "Welding voltage", DataType: core.DataTypeDouble, Unit: "V", InitialValue: 0.0},
		{Name: "WeldTime", DisplayName: "Weld Time", Description: "Current weld duration", DataType: core.DataTypeDouble, Unit: "ms", InitialValue: 0.0},
		{Name: "WeldEnergy", DisplayName: "Weld Energy", Description: "Weld energy", DataType: core.DataTypeDouble, Unit: "J", InitialValue: 0.0},
		{Name: "ElectrodeForce", DisplayName: "Electrode Force", Description: "Electrode force", DataType: core.DataTypeDouble, Unit: "kN", InitialValue: 0.0},
		{Name: "ClampForce", DisplayName: "Clamp Force", Description: "Fixture clamp force", DataType: core.DataTypeDouble, Unit: "kN", InitialValue: 0.0},
		{Name: "ElectrodeTemp", DisplayName: "Electrode Temperature", Description: "Electrode temperature", DataType: core.DataTypeDouble, Unit: "°C", InitialValue: 25.0},
		{Name: "PartTemp", DisplayName: "Part Temperature", Description: "Part temperature", DataType: core.DataTypeDouble, Unit: "°C", InitialValue: 25.0},
		{Name: "WeldCount", DisplayName: "Weld Count", Description: "Welds in current part", DataType: core.DataTypeInt32, Unit: "", InitialValue: int32(0)},
		{Name: "TotalWelds", DisplayName: "Total Welds", Description: "Total welds performed", DataType: core.DataTypeInt32, Unit: "", InitialValue: int32(0)},
		{Name: "CycleCount", DisplayName: "Cycle Count", Description: "Parts completed", DataType: core.DataTypeInt32, Unit: "", InitialValue: int32(0)},
		{Name: "CycleTime", DisplayName: "Cycle Time", Description: "Current cycle time", DataType: core.DataTypeDouble, Unit: "s", InitialValue: 0.0},
		{Name: "State", DisplayName: "State", Description: "Machine state (0-4)", DataType: core.DataTypeInt32, Unit: "", InitialValue: int32(0)},
		{Name: "GoodParts", DisplayName: "Good Parts", Description: "Good parts count", DataType: core.DataTypeInt32, Unit: "", InitialValue: int32(0)},
		{Name: "ScrapParts", DisplayName: "Scrap Parts", Description: "Scrap parts count", DataType: core.DataTypeInt32, Unit: "", InitialValue: int32(0)},
		{Name: "CurrentOrderId", DisplayName: "Current Order ID", Description: "Active order ID", DataType: core.DataTypeString, Unit: "", InitialValue: ""},
		{Name: "CurrentPartNumber", DisplayName: "Current Part Number", Description: "Active part number", DataType: core.DataTypeString, Unit: "", InitialValue: ""},
		{Name: "CycleProgress", DisplayName: "Cycle Progress", Description: "Progress 0-100%", DataType: core.DataTypeDouble, Unit: "%", InitialValue: 0.0},
		{Name: "CurrentPartId", DisplayName: "Current Part ID", Description: "Part being welded", DataType: core.DataTypeString, Unit: "", InitialValue: ""},
		{Name: "ElectrodeWear", DisplayName: "Electrode Wear", Description: "Electrode wear 0-100%", DataType: core.DataTypeDouble, Unit: "%", InitialValue: 0.0},
		{Name: "ErrorCode", DisplayName: "Error Code", Description: "Current error code", DataType: core.DataTypeString, Unit: "", InitialValue: ""},
		{Name: "ErrorMessage", DisplayName: "Error Message", Description: "Error description", DataType: core.DataTypeString, Unit: "", InitialValue: ""},
	}
}

// GenerateData generates current timeseries data
func (sw *SpotWelder) GenerateData() map[string]interface{} {
	// Calculate phase progress
	phaseProgress := sw.calculatePhaseProgress()

	// Generate spot welder data
	data := sw.tsGen.Generate(sw.State(), sw.welderState.Phase, phaseProgress,
		sw.welderState.ElectrodeTemp, sw.welderState.PartTemp, sw.welderState.WeldsInCurrentPart)

	// Add production data
	data.State = sw.State()
	data.Phase = sw.welderState.Phase
	data.GoodParts, data.ScrapParts = sw.GetCounters()
	data.CycleProgress = sw.GetCycleProgress()
	data.CycleCount = sw.welderState.CycleCount
	data.TotalWelds = sw.welderState.TotalWelds
	data.WeldCount = sw.welderState.WeldsInCurrentPart
	data.CycleTime = sw.ElapsedInCycle().Seconds()
	data.CurrentPartID = sw.welderState.CurrentPartID
	data.ElectrodeWear = sw.GetElectrodeWear()

	if sw.CurrentOrder != nil {
		data.CurrentOrderID = sw.CurrentOrder.OrderID
		data.CurrentPartNumber = sw.CurrentOrder.PartNumber
	}

	if sw.CurrentError != nil {
		data.ErrorCode = sw.CurrentError.Code
		data.ErrorMessage = sw.CurrentError.Message
		data.ErrorTimestamp = sw.CurrentError.OccurredAt
	}

	return data.ToMap()
}

func (sw *SpotWelder) calculatePhaseProgress() float64 {
	cycleTime := sw.Config().CycleTime
	elapsed := sw.ElapsedInCycle()
	cfg := sw.welderConfig

	loadEnd := time.Duration(float64(cycleTime) * cfg.LoadFraction)
	clampEnd := loadEnd + time.Duration(float64(cycleTime)*cfg.ClampFraction)
	preWeldEnd := clampEnd + time.Duration(float64(cycleTime)*cfg.PreWeldFraction)
	weldEnd := preWeldEnd + time.Duration(float64(cycleTime)*cfg.WeldFraction)
	holdEnd := weldEnd + time.Duration(float64(cycleTime)*cfg.HoldFraction)
	releaseEnd := holdEnd + time.Duration(float64(cycleTime)*cfg.ReleaseFraction)

	switch sw.welderState.Phase {
	case PhaseLoad:
		return float64(elapsed) / float64(loadEnd)
	case PhaseClamp:
		return float64(elapsed-loadEnd) / float64(clampEnd-loadEnd)
	case PhasePreWeld:
		return float64(elapsed-clampEnd) / float64(preWeldEnd-clampEnd)
	case PhaseWeld:
		return float64(elapsed-preWeldEnd) / float64(weldEnd-preWeldEnd)
	case PhaseHold:
		return float64(elapsed-weldEnd) / float64(holdEnd-weldEnd)
	case PhaseRelease:
		return float64(elapsed-holdEnd) / float64(releaseEnd-holdEnd)
	case PhaseUnload:
		return float64(elapsed-releaseEnd) / float64(cycleTime-releaseEnd)
	default:
		return 0
	}
}

// GetSpotWelderData returns fully populated spot welder data struct
func (sw *SpotWelder) GetSpotWelderData() *SpotWelderData {
	phaseProgress := sw.calculatePhaseProgress()
	data := sw.tsGen.Generate(sw.State(), sw.welderState.Phase, phaseProgress,
		sw.welderState.ElectrodeTemp, sw.welderState.PartTemp, sw.welderState.WeldsInCurrentPart)

	// Add production data
	data.State = sw.State()
	data.Phase = sw.welderState.Phase
	data.GoodParts, data.ScrapParts = sw.GetCounters()
	data.CycleProgress = sw.GetCycleProgress()
	data.CycleCount = sw.welderState.CycleCount
	data.TotalWelds = sw.welderState.TotalWelds
	data.WeldCount = sw.welderState.WeldsInCurrentPart
	data.CycleTime = sw.ElapsedInCycle().Seconds()
	data.CurrentPartID = sw.welderState.CurrentPartID
	data.ElectrodeWear = sw.GetElectrodeWear()

	if sw.CurrentOrder != nil {
		data.CurrentOrderID = sw.CurrentOrder.OrderID
		data.CurrentPartNumber = sw.CurrentOrder.PartNumber
	}

	if sw.CurrentError != nil {
		data.ErrorCode = sw.CurrentError.Code
		data.ErrorMessage = sw.CurrentError.Message
		data.ErrorTimestamp = sw.CurrentError.OccurredAt
	}

	return data
}
