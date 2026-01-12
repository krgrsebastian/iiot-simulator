package welding

import (
	"math"
	"time"

	"github.com/sebastiankruger/shopfloor-simulator/internal/core"
)

// TimeseriesGenerator generates realistic timeseries values for welding parameters
type TimeseriesGenerator struct {
	noise  *core.NoiseGenerator
	config WeldingConfig

	// Position simulation state
	weldPathProgress float64
}

// NewTimeseriesGenerator creates a new timeseries generator with welding config
func NewTimeseriesGenerator(cfg WeldingConfig) *TimeseriesGenerator {
	return &TimeseriesGenerator{
		noise:  core.NewNoiseGenerator(),
		config: cfg,
	}
}

// Generate generates timeseries data based on current state and phase
func (tg *TimeseriesGenerator) Generate(state core.MachineState, phase WeldPhase, phaseProgress float64) *WeldingData {
	data := &WeldingData{
		State:     state,
		Phase:     phase,
		Timestamp: time.Now(),
	}

	switch state {
	case core.StateRunning:
		tg.generateRunningValues(data, phase, phaseProgress)
	case core.StateSetup:
		tg.generateSetupValues(data)
	default:
		tg.generateIdleValues(data)
	}

	return data
}

func (tg *TimeseriesGenerator) generateRunningValues(data *WeldingData, phase WeldPhase, phaseProgress float64) {
	// Calculate phase multiplier for ramp-up/ramp-down
	var phaseMult float64
	var noiseLevel float64

	switch phase {
	case PhaseRampUp:
		// Exponential ramp-up: 1 - e^(-t/tau)
		phaseMult = tg.noise.RampValue(1.0, phaseProgress, true, 0.15)
		noiseLevel = 0.05 // 5% noise during ramp-up

	case PhaseSteady:
		phaseMult = 1.0
		noiseLevel = 0.02 // 2% noise in steady state

	case PhaseRampDown:
		// Exponential ramp-down: e^(-t/tau)
		phaseMult = tg.noise.RampValue(1.0, phaseProgress, false, 0.15)
		noiseLevel = 0.05 // 5% noise during ramp-down

	default:
		phaseMult = 0
		noiseLevel = 0
	}

	// Generate correlated current and voltage with noise
	commonFactor := tg.noise.CommonFactor(0.02) // Shared variance for correlation

	// Current with Gaussian noise and occasional spikes
	baseTarget := tg.config.TargetCurrent * phaseMult
	data.WeldingCurrent = tg.noise.CorrelatedNoise(baseTarget, noiseLevel, commonFactor, 1.0)

	// Add occasional spikes (0.3% chance)
	data.WeldingCurrent += tg.noise.Spike(tg.config.TargetCurrent, 0.003, 0.10)

	// Ensure non-negative
	data.WeldingCurrent = core.ClampPositive(data.WeldingCurrent)

	// Voltage correlated with current (0.4-0.6 correlation)
	baseVoltage := tg.config.TargetVoltage * phaseMult
	data.Voltage = tg.noise.CorrelatedNoise(baseVoltage, noiseLevel*0.5, commonFactor, 0.5)
	data.Voltage = core.ClampPositive(data.Voltage)

	// Wire feed speed - very stable during running
	baseWireFeed := tg.config.TargetWireFeedSpeed * phaseMult
	data.WireFeedSpeed = tg.noise.GaussianNoise(baseWireFeed, 0.005) // Only 0.5% noise
	data.WireFeedSpeed = core.ClampPositive(data.WireFeedSpeed)

	// Gas flow - essentially constant, minimal noise
	// Gas keeps flowing during all weld phases
	if phase == PhaseRampUp || phase == PhaseSteady || phase == PhaseRampDown {
		data.GasFlow = tg.noise.GaussianNoise(tg.config.TargetGasFlow, 0.003) // 0.3% noise
	} else {
		data.GasFlow = 0
	}

	// Travel speed - follows weld path
	baseTravelSpeed := tg.config.TargetTravelSpeed * phaseMult
	data.TravelSpeed = tg.noise.GaussianNoise(baseTravelSpeed, 0.02)
	data.TravelSpeed = core.ClampPositive(data.TravelSpeed)

	// Position simulation - follow a simple linear path
	tg.weldPathProgress += data.TravelSpeed * 0.001 // Progress per ms
	if tg.weldPathProgress > tg.config.WeldPathLength {
		tg.weldPathProgress = 0 // Reset for next weld
	}

	// Simple X-Y motion along a line with some variation
	progress := tg.weldPathProgress / tg.config.WeldPathLength
	data.PositionX = -250 + progress*500 + tg.noise.Gaussian(0, 2)
	data.PositionY = math.Sin(progress*math.Pi*4)*50 + tg.noise.Gaussian(0, 2) // Slight wave pattern
	data.PositionZ = 200 + math.Sin(progress*math.Pi*2)*20 + tg.noise.Gaussian(0, 1)

	// Torch angle varies during weld
	data.TorchAngle = 30 + math.Sin(progress*math.Pi*2)*10 + tg.noise.Gaussian(0, 2)
}

func (tg *TimeseriesGenerator) generateSetupValues(data *WeldingData) {
	// During setup, robot moves to position but no welding
	data.WeldingCurrent = 0
	data.Voltage = 0
	data.WireFeedSpeed = 0
	data.GasFlow = 0 // Gas off during setup
	data.TravelSpeed = 0

	// Robot at home position with slight movement
	data.PositionX = tg.noise.Gaussian(0, 5)
	data.PositionY = tg.noise.Gaussian(0, 5)
	data.PositionZ = 200 + tg.noise.Gaussian(0, 2)
	data.TorchAngle = 0

	// Reset weld path for next cycle
	tg.weldPathProgress = 0
}

func (tg *TimeseriesGenerator) generateIdleValues(data *WeldingData) {
	// All values at zero/home position
	data.WeldingCurrent = 0
	data.Voltage = 0
	data.WireFeedSpeed = 0
	data.GasFlow = 0
	data.TravelSpeed = 0

	// Home position
	data.PositionX = 0
	data.PositionY = 0
	data.PositionZ = 200
	data.TorchAngle = 0

	// Reset weld path
	tg.weldPathProgress = 0
}

// SetTargets allows updating the target welding parameters
func (tg *TimeseriesGenerator) SetTargets(current, voltage, wireFeed, gasFlow, travelSpeed float64) {
	tg.config.TargetCurrent = current
	tg.config.TargetVoltage = voltage
	tg.config.TargetWireFeedSpeed = wireFeed
	tg.config.TargetGasFlow = gasFlow
	tg.config.TargetTravelSpeed = travelSpeed
}
