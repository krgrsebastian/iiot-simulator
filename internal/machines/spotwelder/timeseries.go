package spotwelder

import (
	"time"

	"github.com/sebastiankruger/shopfloor-simulator/internal/core"
)

// TimeseriesGenerator generates realistic timeseries data for a spot welder
type TimeseriesGenerator struct {
	config SpotWelderConfig
	noise  *core.NoiseGenerator

	// Internal state for smooth transitions
	lastCurrent       float64
	lastVoltage       float64
	lastElectrodeForce float64
	lastClampForce    float64
}

// NewTimeseriesGenerator creates a new timeseries generator for spot welders
func NewTimeseriesGenerator(cfg SpotWelderConfig) *TimeseriesGenerator {
	return &TimeseriesGenerator{
		config:            cfg,
		noise:             core.NewNoiseGenerator(),
		lastCurrent:       0,
		lastVoltage:       0,
		lastElectrodeForce: 0,
		lastClampForce:    0,
	}
}

// Generate generates timeseries data based on current state, phase, and temperatures
func (tg *TimeseriesGenerator) Generate(state core.MachineState, phase SpotWelderPhase, phaseProgress float64,
	electrodeTemp, partTemp float64, weldsInPart int) *SpotWelderData {

	data := &SpotWelderData{
		ElectrodeTemp: electrodeTemp,
		PartTemp:      partTemp,
		WeldCount:     weldsInPart,
		Timestamp:     time.Now(),
	}

	switch state {
	case core.StateIdle:
		tg.generateIdleData(data)
	case core.StateSetup:
		tg.generateSetupData(data, phaseProgress)
	case core.StateRunning:
		tg.generateRunningData(data, phase, phaseProgress)
	case core.StatePlannedStop, core.StateUnplannedStop:
		tg.generateStoppedData(data)
	case core.StateWaiting:
		tg.generateIdleData(data) // Waiting behaves like idle for timeseries
	}

	// Add temperature noise (2% variation)
	data.ElectrodeTemp = tg.noise.GaussianNoise(electrodeTemp, 0.02)
	data.PartTemp = tg.noise.GaussianNoise(partTemp, 0.02)

	// Store last values for smooth transitions
	tg.lastCurrent = data.WeldCurrent
	tg.lastVoltage = data.WeldVoltage
	tg.lastElectrodeForce = data.ElectrodeForce
	tg.lastClampForce = data.ClampForce

	return data
}

func (tg *TimeseriesGenerator) generateIdleData(data *SpotWelderData) {
	// All parameters at zero/minimal when idle
	data.WeldCurrent = 0
	data.WeldVoltage = 0
	data.WeldTime = 0
	data.WeldEnergy = 0
	data.ElectrodeForce = 0
	data.ClampForce = 0
}

func (tg *TimeseriesGenerator) generateSetupData(data *SpotWelderData, progress float64) {
	// During setup, systems checking and warming up
	data.WeldCurrent = 0
	data.WeldVoltage = tg.noise.GaussianNoise(0.5, 0.1) // Standby voltage
	data.WeldTime = 0
	data.WeldEnergy = 0

	// Clamps testing
	data.ClampForce = tg.noise.RampValue(tg.config.MaxClampForce*0.3, progress, true, 3.0)
	data.ElectrodeForce = 0
}

func (tg *TimeseriesGenerator) generateRunningData(data *SpotWelderData, phase SpotWelderPhase, phaseProgress float64) {
	switch phase {
	case PhaseLoad:
		tg.generateLoadPhase(data, phaseProgress)
	case PhaseClamp:
		tg.generateClampPhase(data, phaseProgress)
	case PhasePreWeld:
		tg.generatePreWeldPhase(data, phaseProgress)
	case PhaseWeld:
		tg.generateWeldPhase(data, phaseProgress)
	case PhaseHold:
		tg.generateHoldPhase(data, phaseProgress)
	case PhaseRelease:
		tg.generateReleasePhase(data, phaseProgress)
	case PhaseUnload:
		tg.generateUnloadPhase(data, phaseProgress)
	default:
		tg.generateIdleData(data)
	}
}

func (tg *TimeseriesGenerator) generateLoadPhase(data *SpotWelderData, progress float64) {
	// Part being loaded - minimal activity
	data.WeldCurrent = 0
	data.WeldVoltage = tg.noise.GaussianNoise(0.5, 0.1)
	data.WeldTime = 0
	data.WeldEnergy = 0
	data.ElectrodeForce = 0
	data.ClampForce = 0
}

func (tg *TimeseriesGenerator) generateClampPhase(data *SpotWelderData, progress float64) {
	// Fixtures clamping the part
	data.WeldCurrent = 0
	data.WeldVoltage = tg.noise.GaussianNoise(0.5, 0.1)
	data.WeldTime = 0
	data.WeldEnergy = 0
	data.ElectrodeForce = 0

	// Clamp force building
	data.ClampForce = tg.noise.RampValue(tg.config.MaxClampForce, progress, true, 2.0)
	data.ClampForce = tg.noise.GaussianNoise(data.ClampForce, 0.03)
	if data.ClampForce < 0 {
		data.ClampForce = 0
	}
}

func (tg *TimeseriesGenerator) generatePreWeldPhase(data *SpotWelderData, progress float64) {
	// Positioning electrode, applying initial force
	data.WeldCurrent = 0
	data.WeldVoltage = tg.noise.GaussianNoise(0.8, 0.1) // Pre-weld voltage
	data.WeldTime = 0
	data.WeldEnergy = 0

	// Full clamp force (2% noise)
	data.ClampForce = tg.noise.ColoredNoise("clamp", tg.config.MaxClampForce, 0.02, 0.7)

	// Electrode approaching
	data.ElectrodeForce = tg.noise.RampValue(tg.config.MaxElectrodeForce*0.8, progress, true, 2.0)
	data.ElectrodeForce = tg.noise.GaussianNoise(data.ElectrodeForce, 0.02)
	if data.ElectrodeForce < 0 {
		data.ElectrodeForce = 0
	}
}

func (tg *TimeseriesGenerator) generateWeldPhase(data *SpotWelderData, progress float64) {
	// Active welding - current flowing
	// Each weld is a pulse, so we model this with a pulsing pattern
	weldsPerPart := tg.config.WeldsPerPart
	progressPerWeld := 1.0 / float64(weldsPerPart)

	// Which weld are we on?
	currentWeldNum := int(progress / progressPerWeld)
	if currentWeldNum >= weldsPerPart {
		currentWeldNum = weldsPerPart - 1
	}

	// Progress within current weld (0-1)
	weldProgress := (progress - float64(currentWeldNum)*progressPerWeld) / progressPerWeld

	// Weld pulse profile: ramp up (10%), steady (80%), ramp down (10%)
	if weldProgress < 0.1 {
		// Ramp up
		rampProgress := weldProgress / 0.1
		data.WeldCurrent = tg.config.TargetCurrent * rampProgress
		data.WeldVoltage = tg.config.TargetVoltage * rampProgress
	} else if weldProgress < 0.9 {
		// Steady - use 3% noise for current, 2% for voltage
		data.WeldCurrent = tg.noise.ColoredNoise("current", tg.config.TargetCurrent, 0.03, 0.6)
		data.WeldVoltage = tg.noise.ColoredNoise("voltage", tg.config.TargetVoltage, 0.02, 0.6)
	} else {
		// Ramp down
		rampProgress := (weldProgress - 0.9) / 0.1
		data.WeldCurrent = tg.config.TargetCurrent * (1 - rampProgress)
		data.WeldVoltage = tg.config.TargetVoltage * (1 - rampProgress)
	}

	// Weld time accumulates within the weld
	weldDurationMs := float64(tg.config.WeldDuration.Milliseconds())
	data.WeldTime = weldProgress * weldDurationMs

	// Energy = current * voltage * time (simplified)
	data.WeldEnergy = data.WeldCurrent * 1000 * data.WeldVoltage * (data.WeldTime / 1000)

	// Full force during welding (2% noise)
	data.ClampForce = tg.noise.ColoredNoise("clamp", tg.config.MaxClampForce, 0.02, 0.7)
	data.ElectrodeForce = tg.noise.ColoredNoise("electrode", tg.config.MaxElectrodeForce, 0.02, 0.7)
}

func (tg *TimeseriesGenerator) generateHoldPhase(data *SpotWelderData, progress float64) {
	// Post-weld hold - current off, force maintained for cooling
	data.WeldCurrent = tg.noise.RampValue(0, progress*2, true, 1.0) // Quick current drop
	if data.WeldCurrent < 0 {
		data.WeldCurrent = 0
	}
	data.WeldVoltage = tg.noise.GaussianNoise(0.5, 0.1)
	data.WeldTime = 0
	data.WeldEnergy = 0

	// Forces maintained then slowly releasing
	holdProgress := progress
	data.ClampForce = tg.config.MaxClampForce * (1 - holdProgress*0.3)
	data.ClampForce = tg.noise.GaussianNoise(data.ClampForce, 0.02)

	data.ElectrodeForce = tg.config.MaxElectrodeForce * (1 - holdProgress*0.5)
	data.ElectrodeForce = tg.noise.GaussianNoise(data.ElectrodeForce, 0.02)
}

func (tg *TimeseriesGenerator) generateReleasePhase(data *SpotWelderData, progress float64) {
	// Releasing clamps and electrode
	data.WeldCurrent = 0
	data.WeldVoltage = tg.noise.GaussianNoise(0.3, 0.1)
	data.WeldTime = 0
	data.WeldEnergy = 0

	// Forces releasing
	data.ClampForce = tg.config.MaxClampForce * 0.7 * (1 - progress)
	data.ClampForce = tg.noise.GaussianNoise(data.ClampForce, 0.02)
	if data.ClampForce < 0 {
		data.ClampForce = 0
	}

	data.ElectrodeForce = tg.config.MaxElectrodeForce * 0.5 * (1 - progress)
	data.ElectrodeForce = tg.noise.GaussianNoise(data.ElectrodeForce, 0.015)
	if data.ElectrodeForce < 0 {
		data.ElectrodeForce = 0
	}
}

func (tg *TimeseriesGenerator) generateUnloadPhase(data *SpotWelderData, progress float64) {
	// Part being unloaded - all parameters at minimum
	data.WeldCurrent = 0
	data.WeldVoltage = tg.noise.GaussianNoise(0.2, 0.1)
	data.WeldTime = 0
	data.WeldEnergy = 0
	data.ElectrodeForce = 0
	data.ClampForce = 0
}

func (tg *TimeseriesGenerator) generateStoppedData(data *SpotWelderData) {
	// Similar to idle - all off
	data.WeldCurrent = 0
	data.WeldVoltage = tg.noise.GaussianNoise(0.1, 0.05)
	data.WeldTime = 0
	data.WeldEnergy = 0

	// Forces slowly bleeding off
	data.ElectrodeForce = tg.lastElectrodeForce * 0.95
	if data.ElectrodeForce < 0.1 {
		data.ElectrodeForce = 0
	}

	data.ClampForce = tg.lastClampForce * 0.95
	if data.ClampForce < 0.1 {
		data.ClampForce = 0
	}
}
