package picker

import (
	"context"
	"time"

	"github.com/sebastiankruger/shopfloor-simulator/internal/core"
)

// PickerRobot implements MachineSimulator for a 6-axis pick-and-place robot
type PickerRobot struct {
	*core.BaseMachine

	// Picker-specific configuration
	pickerConfig PickerConfig

	// Picker-specific state
	pickerState PickerState

	// Noise generator
	noise *core.NoiseGenerator

	// Timeseries generator
	tsGen *TimeseriesGenerator

	// Machine name
	name string

	// Input buffer (from forming machine)
	inputBuffer *core.PartBuffer

	// Output buffer (to spot welder)
	outputBuffer *core.PartBuffer
}

// NewPickerRobot creates a new picker robot simulator
func NewPickerRobot(name string, cfg core.MachineConfig, pickerCfg PickerConfig) *PickerRobot {
	pr := &PickerRobot{
		BaseMachine:  core.NewBaseMachine(cfg),
		pickerConfig: pickerCfg,
		pickerState: PickerState{
			Phase:           PhaseIdle,
			CurrentPosition: pickerCfg.HomePosition,
			TargetPosition:  pickerCfg.HomePosition,
			Gripper:         GripperOpen,
			GripperPosition: 0,
		},
		noise:        core.NewNoiseGenerator(),
		name:         name,
		inputBuffer:  nil, // Set by coordinator
		outputBuffer: nil, // Set by coordinator
	}

	// Create timeseries generator
	pr.tsGen = NewTimeseriesGenerator(pickerCfg)

	return pr
}

// SetInputBuffer sets the input buffer (parts from forming machine)
func (pr *PickerRobot) SetInputBuffer(buf *core.PartBuffer) {
	pr.inputBuffer = buf
}

// SetOutputBuffer sets the output buffer (parts to spot welder)
func (pr *PickerRobot) SetOutputBuffer(buf *core.PartBuffer) {
	pr.outputBuffer = buf
}

// Name returns the machine name
func (pr *PickerRobot) Name() string {
	return pr.name
}

// MachineType returns "picker"
func (pr *PickerRobot) MachineType() string {
	return "picker"
}

// Start starts the picker robot
func (pr *PickerRobot) Start(ctx context.Context) error {
	pr.pickerState.Phase = PhaseIdle
	pr.pickerState.CycleCount = 0
	pr.pickerState.CurrentPosition = pr.pickerConfig.HomePosition
	pr.pickerState.Gripper = GripperOpen
	pr.pickerState.GripperPosition = 0
	return nil
}

// Stop stops the picker robot
func (pr *PickerRobot) Stop(ctx context.Context) error {
	return nil
}

// Phase returns the current picker phase
func (pr *PickerRobot) Phase() PickerPhase {
	return pr.pickerState.Phase
}

// SetPhase sets the current picker phase
func (pr *PickerRobot) SetPhase(phase PickerPhase) {
	pr.pickerState.Phase = phase
	pr.pickerState.PhaseStartedAt = time.Now()
}

// GetCycleCount returns the cycle count
func (pr *PickerRobot) GetCycleCount() int {
	return pr.pickerState.CycleCount
}

// GetHeldPartID returns the ID of the part currently held
func (pr *PickerRobot) GetHeldPartID() string {
	return pr.pickerState.HeldPartID
}

// Update is called every tick to update the state machine
func (pr *PickerRobot) Update(now time.Time, isBreakTime bool) {
	elapsed := pr.ElapsedInState()

	switch pr.State() {
	case core.StateIdle:
		pr.updateIdle(now)

	case core.StateSetup:
		pr.updateSetup(elapsed, now)

	case core.StateRunning:
		pr.updateRunning(now, isBreakTime)

	case core.StatePlannedStop:
		pr.updatePlannedStop(isBreakTime)

	case core.StateUnplannedStop:
		pr.updateUnplannedStop(now)
	}
}

func (pr *PickerRobot) updateIdle(now time.Time) {
	pr.pickerState.Phase = PhaseIdle
	pr.pickerState.CurrentPosition = pr.pickerConfig.HomePosition

	// Check if there's a part to pick up
	if pr.inputBuffer != nil && pr.inputBuffer.Count() > 0 {
		pr.TransitionTo(core.StateSetup)
	}
}

func (pr *PickerRobot) updateSetup(elapsed time.Duration, now time.Time) {
	// Quick setup - just verify position
	if elapsed >= pr.Config().GetEffectiveSetupTime() {
		pr.TransitionTo(core.StateRunning)
		pr.CycleStartedAt = now
		pr.SetPhase(PhaseMoveToPickup)
		pr.pickerState.TargetPosition = Position3D{
			X: pr.pickerConfig.PickupPosition.X,
			Y: pr.pickerConfig.PickupPosition.Y,
			Z: pr.pickerConfig.SafeZ,
		}
	}
}

func (pr *PickerRobot) updateRunning(now time.Time, isBreakTime bool) {
	// Check for break time - only stop if not holding a part
	if isBreakTime && pr.pickerState.HeldPartID == "" {
		pr.TransitionTo(core.StatePlannedStop)
		pr.pickerState.Phase = PhaseIdle
		return
	}

	// Check for random error
	if pr.shouldTriggerError() {
		pr.triggerError(now)
		return
	}

	// Update picker phase
	cycleElapsed := pr.ElapsedInCycle()
	cycleTime := pr.Config().GetEffectiveCycleTime()

	// Calculate phase boundaries
	cfg := pr.pickerConfig
	moveToPickupEnd := time.Duration(float64(cycleTime) * cfg.MoveToPickupFraction)
	approachPickupEnd := moveToPickupEnd + time.Duration(float64(cycleTime)*cfg.ApproachPickupFraction)
	gripEnd := approachPickupEnd + time.Duration(float64(cycleTime)*cfg.GripFraction)
	retractPickupEnd := gripEnd + time.Duration(float64(cycleTime)*cfg.RetractPickupFraction)
	moveToPlaceEnd := retractPickupEnd + time.Duration(float64(cycleTime)*cfg.MoveToPlaceFraction)
	approachPlaceEnd := moveToPlaceEnd + time.Duration(float64(cycleTime)*cfg.ApproachPlaceFraction)
	releaseEnd := approachPlaceEnd + time.Duration(float64(cycleTime)*cfg.ReleaseFraction)
	retractPlaceEnd := releaseEnd + time.Duration(float64(cycleTime)*cfg.RetractPlaceFraction)

	switch pr.pickerState.Phase {
	case PhaseMoveToPickup:
		pr.updateMoveToPickup(cycleElapsed, moveToPickupEnd, now)

	case PhaseApproachPickup:
		pr.updateApproachPickup(cycleElapsed, moveToPickupEnd, approachPickupEnd)

	case PhaseGrip:
		pr.updateGrip(cycleElapsed, approachPickupEnd, gripEnd, now)

	case PhaseRetractPickup:
		pr.updateRetractPickup(cycleElapsed, gripEnd, retractPickupEnd)

	case PhaseMoveToPlace:
		pr.updateMoveToPlace(cycleElapsed, retractPickupEnd, moveToPlaceEnd)

	case PhaseApproachPlace:
		pr.updateApproachPlace(cycleElapsed, moveToPlaceEnd, approachPlaceEnd)

	case PhaseRelease:
		pr.updateRelease(cycleElapsed, approachPlaceEnd, releaseEnd, now)

	case PhaseRetractPlace:
		pr.updateRetractPlace(cycleElapsed, releaseEnd, retractPlaceEnd, now)
	}
}

func (pr *PickerRobot) updateMoveToPickup(elapsed, phaseEnd time.Duration, now time.Time) {
	progress := float64(elapsed) / float64(phaseEnd)
	if progress > 1 {
		progress = 1
	}

	// Interpolate position from current to pickup (at safe Z)
	pr.pickerState.CurrentPosition = pr.interpolatePosition(
		pr.pickerConfig.HomePosition,
		Position3D{
			X: pr.pickerConfig.PickupPosition.X,
			Y: pr.pickerConfig.PickupPosition.Y,
			Z: pr.pickerConfig.SafeZ,
		},
		progress,
	)

	if elapsed >= phaseEnd {
		pr.SetPhase(PhaseApproachPickup)
	}
}

func (pr *PickerRobot) updateApproachPickup(elapsed, phaseStart, phaseEnd time.Duration) {
	phaseElapsed := elapsed - phaseStart
	phaseDuration := phaseEnd - phaseStart
	progress := float64(phaseElapsed) / float64(phaseDuration)
	if progress > 1 {
		progress = 1
	}

	// Descend to pickup position
	pr.pickerState.CurrentPosition = pr.interpolatePosition(
		Position3D{
			X: pr.pickerConfig.PickupPosition.X,
			Y: pr.pickerConfig.PickupPosition.Y,
			Z: pr.pickerConfig.SafeZ,
		},
		pr.pickerConfig.PickupPosition,
		progress,
	)

	if elapsed >= phaseEnd {
		pr.SetPhase(PhaseGrip)
		pr.pickerState.Gripper = GripperClosing
	}
}

func (pr *PickerRobot) updateGrip(elapsed, phaseStart, phaseEnd time.Duration, now time.Time) {
	phaseElapsed := elapsed - phaseStart
	phaseDuration := phaseEnd - phaseStart
	progress := float64(phaseElapsed) / float64(phaseDuration)
	if progress > 1 {
		progress = 1
	}

	// Gripper closing
	pr.pickerState.GripperPosition = progress * 100

	if elapsed >= phaseEnd {
		pr.pickerState.Gripper = GripperClosed
		pr.pickerState.GripperPosition = 100

		// Pick up part from input buffer
		if pr.inputBuffer != nil {
			if part := pr.inputBuffer.Pop(); part != nil {
				pr.pickerState.HeldPartID = part.ID
				pr.pickerState.HeldPart = part
				part.Status = core.PartStatusInTransit
				part.Location = pr.name
			}
		}

		pr.SetPhase(PhaseRetractPickup)
	}
}

func (pr *PickerRobot) updateRetractPickup(elapsed, phaseStart, phaseEnd time.Duration) {
	phaseElapsed := elapsed - phaseStart
	phaseDuration := phaseEnd - phaseStart
	progress := float64(phaseElapsed) / float64(phaseDuration)
	if progress > 1 {
		progress = 1
	}

	// Retract to safe Z
	pr.pickerState.CurrentPosition = pr.interpolatePosition(
		pr.pickerConfig.PickupPosition,
		Position3D{
			X: pr.pickerConfig.PickupPosition.X,
			Y: pr.pickerConfig.PickupPosition.Y,
			Z: pr.pickerConfig.SafeZ,
		},
		progress,
	)

	if elapsed >= phaseEnd {
		pr.SetPhase(PhaseMoveToPlace)
	}
}

func (pr *PickerRobot) updateMoveToPlace(elapsed, phaseStart, phaseEnd time.Duration) {
	phaseElapsed := elapsed - phaseStart
	phaseDuration := phaseEnd - phaseStart
	progress := float64(phaseElapsed) / float64(phaseDuration)
	if progress > 1 {
		progress = 1
	}

	// Move horizontally to place position
	pr.pickerState.CurrentPosition = pr.interpolatePosition(
		Position3D{
			X: pr.pickerConfig.PickupPosition.X,
			Y: pr.pickerConfig.PickupPosition.Y,
			Z: pr.pickerConfig.SafeZ,
		},
		Position3D{
			X: pr.pickerConfig.PlacePosition.X,
			Y: pr.pickerConfig.PlacePosition.Y,
			Z: pr.pickerConfig.SafeZ,
		},
		progress,
	)

	if elapsed >= phaseEnd {
		pr.SetPhase(PhaseApproachPlace)
	}
}

func (pr *PickerRobot) updateApproachPlace(elapsed, phaseStart, phaseEnd time.Duration) {
	phaseElapsed := elapsed - phaseStart
	phaseDuration := phaseEnd - phaseStart
	progress := float64(phaseElapsed) / float64(phaseDuration)
	if progress > 1 {
		progress = 1
	}

	// Descend to place position
	pr.pickerState.CurrentPosition = pr.interpolatePosition(
		Position3D{
			X: pr.pickerConfig.PlacePosition.X,
			Y: pr.pickerConfig.PlacePosition.Y,
			Z: pr.pickerConfig.SafeZ,
		},
		pr.pickerConfig.PlacePosition,
		progress,
	)

	if elapsed >= phaseEnd {
		pr.SetPhase(PhaseRelease)
		pr.pickerState.Gripper = GripperOpening
	}
}

func (pr *PickerRobot) updateRelease(elapsed, phaseStart, phaseEnd time.Duration, now time.Time) {
	phaseElapsed := elapsed - phaseStart
	phaseDuration := phaseEnd - phaseStart
	progress := float64(phaseElapsed) / float64(phaseDuration)
	if progress > 1 {
		progress = 1
	}

	// Gripper opening
	pr.pickerState.GripperPosition = (1 - progress) * 100

	if elapsed >= phaseEnd {
		pr.pickerState.Gripper = GripperOpen
		pr.pickerState.GripperPosition = 0

		// Release part to output buffer
		if pr.pickerState.HeldPart != nil {
			pr.pickerState.HeldPart.Status = core.PartStatusAwaitingWelding
			pr.pickerState.HeldPart.PickingComplete = now
			pr.pickerState.HeldPart.PickerRobotID = pr.name

			if pr.outputBuffer != nil {
				pr.outputBuffer.Push(pr.pickerState.HeldPart)
			}

			pr.pickerState.HeldPartID = ""
			pr.pickerState.HeldPart = nil
		}

		pr.SetPhase(PhaseRetractPlace)
	}
}

func (pr *PickerRobot) updateRetractPlace(elapsed, phaseStart, phaseEnd time.Duration, now time.Time) {
	phaseElapsed := elapsed - phaseStart
	phaseDuration := phaseEnd - phaseStart
	progress := float64(phaseElapsed) / float64(phaseDuration)
	if progress > 1 {
		progress = 1
	}

	// Retract to safe Z
	pr.pickerState.CurrentPosition = pr.interpolatePosition(
		pr.pickerConfig.PlacePosition,
		Position3D{
			X: pr.pickerConfig.PlacePosition.X,
			Y: pr.pickerConfig.PlacePosition.Y,
			Z: pr.pickerConfig.SafeZ,
		},
		progress,
	)

	if elapsed >= phaseEnd {
		pr.completeCycle(now)
	}
}

func (pr *PickerRobot) interpolatePosition(from, to Position3D, progress float64) Position3D {
	return Position3D{
		X: from.X + (to.X-from.X)*progress,
		Y: from.Y + (to.Y-from.Y)*progress,
		Z: from.Z + (to.Z-from.Z)*progress,
	}
}

func (pr *PickerRobot) updatePlannedStop(isBreakTime bool) {
	if !isBreakTime {
		pr.TransitionTo(core.StateIdle)
	}
}

func (pr *PickerRobot) updateUnplannedStop(now time.Time) {
	if pr.IsErrorResolved(now) {
		pr.ClearError()
		pr.TransitionTo(core.StateIdle)
	}
}

func (pr *PickerRobot) shouldTriggerError() bool {
	// Only trigger errors during movement phases
	if pr.pickerState.Phase == PhaseIdle {
		return false
	}
	return pr.noise.ShouldTrigger(pr.Config().GetEffectiveErrorRate(), pr.Config().PublishInterval, pr.Config().GetEffectiveCycleTime())
}

func (pr *PickerRobot) triggerError(now time.Time) {
	errors := AllErrorCodes()
	errorCode := errors[pr.noise.UniformInt(0, len(errors)-1)]
	message, minDur, maxDur := GetErrorInfo(errorCode)
	baseDuration := time.Duration(pr.noise.Uniform(float64(minDur), float64(maxDur)))
	duration := pr.Config().GetEffectiveErrorDuration(baseDuration)

	// If holding a part and error is part dropped, mark as scrap
	if errorCode == ErrorPartDropped && pr.pickerState.HeldPartID != "" {
		pr.ScrapParts++
		pr.pickerState.HeldPartID = ""
		pr.pickerState.HeldPart = nil
		pr.pickerState.Gripper = GripperOpen
		pr.pickerState.GripperPosition = 0
	}

	pr.TriggerError(string(errorCode), message, duration)
	pr.pickerState.Phase = PhaseIdle
}

func (pr *PickerRobot) completeCycle(now time.Time) {
	pr.pickerState.CycleCount++
	pr.GoodParts++
	pr.CompleteCycle(false)

	// Check if more parts to pick
	if pr.inputBuffer != nil && pr.inputBuffer.Count() > 0 {
		// Start next cycle
		pr.CycleStartedAt = now
		pr.SetPhase(PhaseMoveToPickup)
		pr.pickerState.CurrentPosition = Position3D{
			X: pr.pickerConfig.PlacePosition.X,
			Y: pr.pickerConfig.PlacePosition.Y,
			Z: pr.pickerConfig.SafeZ,
		}
	} else {
		// Return to idle
		pr.TransitionTo(core.StateIdle)
		pr.pickerState.Phase = PhaseIdle
	}
}

// GetCycleProgress returns the current cycle progress (0-100%)
func (pr *PickerRobot) GetCycleProgress() float64 {
	if pr.State() != core.StateRunning {
		return 0
	}
	elapsed := pr.ElapsedInCycle()
	progress := float64(elapsed) / float64(pr.Config().GetEffectiveCycleTime()) * 100
	if progress > 100 {
		progress = 100
	}
	return progress
}

// GetOPCUANodes returns the OPC UA node definitions for a picker robot
func (pr *PickerRobot) GetOPCUANodes() []core.NodeDefinition {
	return []core.NodeDefinition{
		{Name: "PositionX", DisplayName: "Position X", Description: "TCP X position", DataType: core.DataTypeDouble, Unit: "mm", InitialValue: 0.0},
		{Name: "PositionY", DisplayName: "Position Y", Description: "TCP Y position", DataType: core.DataTypeDouble, Unit: "mm", InitialValue: 0.0},
		{Name: "PositionZ", DisplayName: "Position Z", Description: "TCP Z position", DataType: core.DataTypeDouble, Unit: "mm", InitialValue: 0.0},
		{Name: "Speed", DisplayName: "Speed", Description: "TCP speed", DataType: core.DataTypeDouble, Unit: "mm/s", InitialValue: 0.0},
		{Name: "Joint1", DisplayName: "Joint 1", Description: "Base rotation", DataType: core.DataTypeDouble, Unit: "deg", InitialValue: 0.0},
		{Name: "Joint2", DisplayName: "Joint 2", Description: "Shoulder", DataType: core.DataTypeDouble, Unit: "deg", InitialValue: 0.0},
		{Name: "Joint3", DisplayName: "Joint 3", Description: "Elbow", DataType: core.DataTypeDouble, Unit: "deg", InitialValue: 0.0},
		{Name: "Joint4", DisplayName: "Joint 4", Description: "Wrist 1", DataType: core.DataTypeDouble, Unit: "deg", InitialValue: 0.0},
		{Name: "Joint5", DisplayName: "Joint 5", Description: "Wrist 2", DataType: core.DataTypeDouble, Unit: "deg", InitialValue: 0.0},
		{Name: "Joint6", DisplayName: "Joint 6", Description: "Wrist 3", DataType: core.DataTypeDouble, Unit: "deg", InitialValue: 0.0},
		{Name: "GripperState", DisplayName: "Gripper State", Description: "Gripper state (0=Open,1=Closing,2=Closed,3=Opening)", DataType: core.DataTypeInt32, Unit: "", InitialValue: int32(0)},
		{Name: "GripperPosition", DisplayName: "Gripper Position", Description: "Gripper position 0-100%", DataType: core.DataTypeDouble, Unit: "%", InitialValue: 0.0},
		{Name: "GripForce", DisplayName: "Grip Force", Description: "Gripper force", DataType: core.DataTypeDouble, Unit: "N", InitialValue: 0.0},
		{Name: "CycleCount", DisplayName: "Cycle Count", Description: "Total cycles completed", DataType: core.DataTypeInt32, Unit: "", InitialValue: int32(0)},
		{Name: "CycleTime", DisplayName: "Cycle Time", Description: "Current cycle time", DataType: core.DataTypeDouble, Unit: "s", InitialValue: 0.0},
		{Name: "State", DisplayName: "State", Description: "Machine state (0-4)", DataType: core.DataTypeInt32, Unit: "", InitialValue: int32(0)},
		{Name: "GoodParts", DisplayName: "Good Parts", Description: "Good parts count", DataType: core.DataTypeInt32, Unit: "", InitialValue: int32(0)},
		{Name: "ScrapParts", DisplayName: "Scrap Parts", Description: "Scrap parts count", DataType: core.DataTypeInt32, Unit: "", InitialValue: int32(0)},
		{Name: "CurrentOrderId", DisplayName: "Current Order ID", Description: "Active order ID", DataType: core.DataTypeString, Unit: "", InitialValue: ""},
		{Name: "CurrentPartNumber", DisplayName: "Current Part Number", Description: "Active part number", DataType: core.DataTypeString, Unit: "", InitialValue: ""},
		{Name: "CycleProgress", DisplayName: "Cycle Progress", Description: "Progress 0-100%", DataType: core.DataTypeDouble, Unit: "%", InitialValue: 0.0},
		{Name: "PartInGripper", DisplayName: "Part In Gripper", Description: "Part ID being held", DataType: core.DataTypeString, Unit: "", InitialValue: ""},
		{Name: "ErrorCode", DisplayName: "Error Code", Description: "Current error code", DataType: core.DataTypeString, Unit: "", InitialValue: ""},
		{Name: "ErrorMessage", DisplayName: "Error Message", Description: "Error description", DataType: core.DataTypeString, Unit: "", InitialValue: ""},
	}
}

// GenerateData generates current timeseries data
func (pr *PickerRobot) GenerateData() map[string]interface{} {
	// Calculate phase progress
	phaseProgress := pr.calculatePhaseProgress()

	// Generate picker data
	data := pr.tsGen.Generate(pr.State(), pr.pickerState.Phase, phaseProgress,
		pr.pickerState.CurrentPosition, pr.pickerState.Gripper, pr.pickerState.GripperPosition)

	// Add production data
	data.State = pr.State()
	data.Phase = pr.pickerState.Phase
	data.GoodParts, data.ScrapParts = pr.GetCounters()
	data.CycleProgress = pr.GetCycleProgress()
	data.CycleCount = pr.pickerState.CycleCount
	data.CycleTime = pr.ElapsedInCycle().Seconds()
	data.PartInGripper = pr.pickerState.HeldPartID

	if pr.CurrentOrder != nil {
		data.CurrentOrderID = pr.CurrentOrder.OrderID
		data.CurrentPartNumber = pr.CurrentOrder.PartNumber
	}

	if pr.CurrentError != nil {
		data.ErrorCode = pr.CurrentError.Code
		data.ErrorMessage = pr.CurrentError.Message
		data.ErrorTimestamp = pr.CurrentError.OccurredAt
	}

	return data.ToMap()
}

func (pr *PickerRobot) calculatePhaseProgress() float64 {
	cycleTime := pr.Config().GetEffectiveCycleTime()
	elapsed := pr.ElapsedInCycle()
	cfg := pr.pickerConfig

	moveToPickupEnd := time.Duration(float64(cycleTime) * cfg.MoveToPickupFraction)
	approachPickupEnd := moveToPickupEnd + time.Duration(float64(cycleTime)*cfg.ApproachPickupFraction)
	gripEnd := approachPickupEnd + time.Duration(float64(cycleTime)*cfg.GripFraction)
	retractPickupEnd := gripEnd + time.Duration(float64(cycleTime)*cfg.RetractPickupFraction)
	moveToPlaceEnd := retractPickupEnd + time.Duration(float64(cycleTime)*cfg.MoveToPlaceFraction)
	approachPlaceEnd := moveToPlaceEnd + time.Duration(float64(cycleTime)*cfg.ApproachPlaceFraction)
	releaseEnd := approachPlaceEnd + time.Duration(float64(cycleTime)*cfg.ReleaseFraction)
	retractPlaceEnd := releaseEnd + time.Duration(float64(cycleTime)*cfg.RetractPlaceFraction)

	switch pr.pickerState.Phase {
	case PhaseMoveToPickup:
		return float64(elapsed) / float64(moveToPickupEnd)
	case PhaseApproachPickup:
		return float64(elapsed-moveToPickupEnd) / float64(approachPickupEnd-moveToPickupEnd)
	case PhaseGrip:
		return float64(elapsed-approachPickupEnd) / float64(gripEnd-approachPickupEnd)
	case PhaseRetractPickup:
		return float64(elapsed-gripEnd) / float64(retractPickupEnd-gripEnd)
	case PhaseMoveToPlace:
		return float64(elapsed-retractPickupEnd) / float64(moveToPlaceEnd-retractPickupEnd)
	case PhaseApproachPlace:
		return float64(elapsed-moveToPlaceEnd) / float64(approachPlaceEnd-moveToPlaceEnd)
	case PhaseRelease:
		return float64(elapsed-approachPlaceEnd) / float64(releaseEnd-approachPlaceEnd)
	case PhaseRetractPlace:
		return float64(elapsed-releaseEnd) / float64(retractPlaceEnd-releaseEnd)
	default:
		return 0
	}
}

// GetPickerData returns fully populated picker data struct
func (pr *PickerRobot) GetPickerData() *PickerData {
	phaseProgress := pr.calculatePhaseProgress()
	data := pr.tsGen.Generate(pr.State(), pr.pickerState.Phase, phaseProgress,
		pr.pickerState.CurrentPosition, pr.pickerState.Gripper, pr.pickerState.GripperPosition)

	// Add production data
	data.State = pr.State()
	data.Phase = pr.pickerState.Phase
	data.GoodParts, data.ScrapParts = pr.GetCounters()
	data.CycleProgress = pr.GetCycleProgress()
	data.CycleCount = pr.pickerState.CycleCount
	data.CycleTime = pr.ElapsedInCycle().Seconds()
	data.PartInGripper = pr.pickerState.HeldPartID

	if pr.CurrentOrder != nil {
		data.CurrentOrderID = pr.CurrentOrder.OrderID
		data.CurrentPartNumber = pr.CurrentOrder.PartNumber
	}

	if pr.CurrentError != nil {
		data.ErrorCode = pr.CurrentError.Code
		data.ErrorMessage = pr.CurrentError.Message
		data.ErrorTimestamp = pr.CurrentError.OccurredAt
	}

	return data
}
