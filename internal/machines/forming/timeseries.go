package forming

import (
	"time"

	"github.com/sebastiankruger/shopfloor-simulator/internal/core"
)

// TimeseriesGenerator generates realistic timeseries data for a forming machine
type TimeseriesGenerator struct {
	config FormingConfig
	noise  *core.NoiseGenerator

	// Internal state for smooth transitions
	lastTemperature    float64
	lastDieTemperature float64
	lastPressure       float64
	lastFormingForce   float64
	lastRamSpeed       float64
}

// NewTimeseriesGenerator creates a new timeseries generator for forming machines
func NewTimeseriesGenerator(cfg FormingConfig) *TimeseriesGenerator {
	return &TimeseriesGenerator{
		config:             cfg,
		noise:              core.NewNoiseGenerator(),
		lastTemperature:    25.0, // Ambient
		lastDieTemperature: 25.0, // Ambient
		lastPressure:       0.0,
		lastFormingForce:   0.0,
		lastRamSpeed:       0.0,
	}
}

// Generate generates timeseries data based on current state, phase, and ram position
func (tg *TimeseriesGenerator) Generate(state core.MachineState, phase FormingPhase, phaseProgress float64, ramPosition float64) *FormingData {
	data := &FormingData{
		RamPosition: ramPosition,
		Timestamp:   time.Now(),
	}

	switch state {
	case core.StateIdle:
		tg.generateIdleData(data)
	case core.StateSetup:
		tg.generateSetupData(data, phaseProgress)
	case core.StateRunning:
		tg.generateRunningData(data, phase, phaseProgress, ramPosition)
	case core.StatePlannedStop, core.StateUnplannedStop:
		tg.generateStoppedData(data)
	}

	// Store last values for smooth transitions
	tg.lastTemperature = data.Temperature
	tg.lastDieTemperature = data.DieTemperature
	tg.lastPressure = data.Pressure
	tg.lastFormingForce = data.FormingForce
	tg.lastRamSpeed = data.RamSpeed

	return data
}

func (tg *TimeseriesGenerator) generateIdleData(data *FormingData) {
	// Cool down towards ambient
	ambientTemp := 25.0

	// Temperature decays towards ambient
	data.Temperature = tg.noise.ColoredNoise("temp",
		ambientTemp+(tg.lastTemperature-ambientTemp)*0.99,
		0.02, 0.7) // 2% noise

	data.DieTemperature = tg.noise.ColoredNoise("dieTemp",
		ambientTemp+(tg.lastDieTemperature-ambientTemp)*0.98,
		0.02, 0.7) // 2% noise

	// No pressure or force when idle
	data.Pressure = tg.noise.GaussianNoise(0, 0.01)
	data.Pressure = core.ClampPositive(data.Pressure)

	data.FormingForce = 0
	data.RamSpeed = 0
	data.RamPosition = 0
}

func (tg *TimeseriesGenerator) generateSetupData(data *FormingData, progress float64) {
	// During setup, dies are preheating (if hot forming)
	// For cold forming, just minor warmup from hydraulic activity

	targetTemp := tg.config.TargetTemperature * 0.8 // Not fully up to temp yet
	data.Temperature = tg.noise.RampValue(targetTemp, progress, true, 2.0)
	data.Temperature = tg.noise.GaussianNoise(data.Temperature, 0.02) // 2% noise
	data.Temperature = core.Clamp(data.Temperature, 20, 100)

	data.DieTemperature = tg.noise.RampValue(tg.config.TargetTemperature*0.7, progress, true, 2.5)
	data.DieTemperature = tg.noise.GaussianNoise(data.DieTemperature, 0.02) // 2% noise
	data.DieTemperature = core.Clamp(data.DieTemperature, 20, 100)

	// Low hydraulic activity during setup
	data.Pressure = tg.noise.GaussianNoise(tg.config.MaxPressure*0.1, 0.05) // 5% noise
	data.Pressure = core.Clamp(data.Pressure, 0, tg.config.MaxPressure)
	data.FormingForce = 0
	data.RamSpeed = 0
	data.RamPosition = 0
}

func (tg *TimeseriesGenerator) generateRunningData(data *FormingData, phase FormingPhase, phaseProgress float64, ramPosition float64) {
	switch phase {
	case PhaseLoad:
		tg.generateLoadPhase(data, phaseProgress)
	case PhaseForm:
		tg.generateFormPhase(data, phaseProgress, ramPosition)
	case PhaseHold:
		tg.generateHoldPhase(data, phaseProgress)
	case PhaseEject:
		tg.generateEjectPhase(data, phaseProgress)
	case PhaseRaise:
		tg.generateRaisePhase(data, phaseProgress, ramPosition)
	default:
		tg.generateIdleData(data)
	}
}

func (tg *TimeseriesGenerator) generateLoadPhase(data *FormingData, progress float64) {
	// Sheet metal being loaded - dies at operating temp, low pressure
	data.Temperature = tg.noise.ColoredNoise("temp", tg.config.TargetTemperature, 0.02, 0.7) // 2% noise
	data.Temperature = core.Clamp(data.Temperature, 20, 100)
	data.DieTemperature = tg.noise.ColoredNoise("dieTemp", tg.config.TargetTemperature*0.95, 0.02, 0.7) // 2% noise
	data.DieTemperature = core.Clamp(data.DieTemperature, 20, 100)

	// Hydraulic system pressurizing
	data.Pressure = tg.noise.RampValue(tg.config.MaxPressure*0.3, progress, true, 3.0)
	data.Pressure = tg.noise.GaussianNoise(data.Pressure, 0.03) // 3% noise
	data.Pressure = core.Clamp(data.Pressure, 0, tg.config.MaxPressure)

	data.FormingForce = 0
	data.RamSpeed = 0
	data.RamPosition = 0
}

func (tg *TimeseriesGenerator) generateFormPhase(data *FormingData, progress float64, ramPosition float64) {
	// Ram descending, forming the sheet metal
	// Temperature rises due to friction and deformation
	tempRise := progress * 10.0 // Up to 10°C rise during forming
	data.Temperature = tg.noise.ColoredNoise("temp",
		tg.config.TargetTemperature+tempRise, 0.02, 0.6) // 2% noise
	data.Temperature = core.Clamp(data.Temperature, 20, 100)

	data.DieTemperature = tg.noise.ColoredNoise("dieTemp",
		tg.config.TargetTemperature+tempRise*0.7, 0.02, 0.6) // 2% noise
	data.DieTemperature = core.Clamp(data.DieTemperature, 20, 100)

	// Pressure builds as ram descends
	pressureProgress := progress * progress // Quadratic buildup
	data.Pressure = tg.noise.ColoredNoise("pressure",
		tg.config.MaxPressure*pressureProgress, 0.03, 0.5) // 3% noise
	data.Pressure = core.Clamp(data.Pressure, 0, tg.config.MaxPressure*1.1)

	// Forming force increases with depth
	data.FormingForce = tg.noise.ColoredNoise("force",
		tg.config.MaxFormingForce*progress, 0.03, 0.5) // 3% noise
	data.FormingForce = core.ClampPositive(data.FormingForce)

	// Ram speed - fast at start, slowing as force builds
	speedProfile := (1 - progress*0.5) // Slows to 50% at end
	data.RamSpeed = tg.noise.GaussianNoise(tg.config.MaxRamSpeed*speedProfile, 0.05) // 5% noise
	data.RamSpeed = core.ClampPositive(data.RamSpeed)

	data.RamPosition = ramPosition
}

func (tg *TimeseriesGenerator) generateHoldPhase(data *FormingData, progress float64) {
	// Holding pressure at bottom - max force
	// Temperature slightly rising from sustained pressure
	tempRise := 10.0 + progress*2.0 // Additional 2°C during hold
	data.Temperature = tg.noise.ColoredNoise("temp",
		tg.config.TargetTemperature+tempRise, 0.02, 0.7) // 2% noise
	data.Temperature = core.Clamp(data.Temperature, 20, 100)

	data.DieTemperature = tg.noise.ColoredNoise("dieTemp",
		tg.config.TargetTemperature+tempRise*0.8, 0.02, 0.7) // 2% noise
	data.DieTemperature = core.Clamp(data.DieTemperature, 20, 100)

	// Full pressure maintained
	data.Pressure = tg.noise.ColoredNoise("pressure",
		tg.config.MaxPressure, 0.02, 0.6) // 2% noise
	data.Pressure = core.Clamp(data.Pressure, 0, tg.config.MaxPressure*1.1)

	// Full forming force maintained
	data.FormingForce = tg.noise.ColoredNoise("force",
		tg.config.MaxFormingForce, 0.02, 0.6) // 2% noise
	data.FormingForce = core.ClampPositive(data.FormingForce)

	// Ram stationary
	data.RamSpeed = tg.noise.GaussianNoise(0, 0.01)
	data.RamPosition = tg.config.RamTravel
}

func (tg *TimeseriesGenerator) generateEjectPhase(data *FormingData, progress float64) {
	// Part ejection - pressure releasing
	data.Temperature = tg.noise.ColoredNoise("temp",
		tg.config.TargetTemperature+8.0*(1-progress), 0.02, 0.7) // 2% noise
	data.Temperature = core.Clamp(data.Temperature, 20, 100)

	data.DieTemperature = tg.noise.ColoredNoise("dieTemp",
		tg.config.TargetTemperature+5.0*(1-progress), 0.02, 0.7) // 2% noise
	data.DieTemperature = core.Clamp(data.DieTemperature, 20, 100)

	// Pressure releasing
	data.Pressure = tg.config.MaxPressure * (1 - progress)
	data.Pressure = tg.noise.GaussianNoise(data.Pressure, 0.03) // 3% noise
	data.Pressure = core.ClampPositive(data.Pressure)

	// Force releasing
	data.FormingForce = tg.config.MaxFormingForce * (1 - progress)
	data.FormingForce = core.ClampPositive(data.FormingForce)

	data.RamSpeed = 0
	data.RamPosition = tg.config.RamTravel
}

func (tg *TimeseriesGenerator) generateRaisePhase(data *FormingData, progress float64, ramPosition float64) {
	// Ram returning to top
	data.Temperature = tg.noise.ColoredNoise("temp",
		tg.config.TargetTemperature+5.0*(1-progress), 0.02, 0.7) // 2% noise
	data.Temperature = core.Clamp(data.Temperature, 20, 100)

	data.DieTemperature = tg.noise.ColoredNoise("dieTemp",
		tg.config.TargetTemperature+3.0*(1-progress), 0.02, 0.7) // 2% noise
	data.DieTemperature = core.Clamp(data.DieTemperature, 20, 100)

	// Low pressure for return stroke
	data.Pressure = tg.noise.GaussianNoise(tg.config.MaxPressure*0.15, 0.05) // 5% noise
	data.Pressure = core.ClampPositive(data.Pressure)

	data.FormingForce = 0

	// Ram speed - fast return (negative = going up)
	data.RamSpeed = tg.noise.GaussianNoise(-tg.config.MaxRamSpeed*0.8, 0.05) // 5% noise

	data.RamPosition = ramPosition
}

func (tg *TimeseriesGenerator) generateStoppedData(data *FormingData) {
	// Similar to idle but maintaining some residual heat
	ambientTemp := 25.0

	// Slower cooldown when stopped mid-operation
	data.Temperature = tg.noise.ColoredNoise("temp",
		ambientTemp+(tg.lastTemperature-ambientTemp)*0.995,
		0.02, 0.7) // 2% noise
	data.Temperature = core.Clamp(data.Temperature, 20, 100)

	data.DieTemperature = tg.noise.ColoredNoise("dieTemp",
		ambientTemp+(tg.lastDieTemperature-ambientTemp)*0.99,
		0.02, 0.7) // 2% noise
	data.DieTemperature = core.Clamp(data.DieTemperature, 20, 100)

	// Hydraulic pressure bleeds off slowly
	data.Pressure = tg.lastPressure * 0.98
	if data.Pressure < 1.0 {
		data.Pressure = 0
	}
	data.Pressure = tg.noise.GaussianNoise(data.Pressure, 0.02) // 2% noise
	data.Pressure = core.ClampPositive(data.Pressure)

	data.FormingForce = tg.lastFormingForce * 0.95
	if data.FormingForce < 1.0 {
		data.FormingForce = 0
	}

	data.RamSpeed = 0
}
