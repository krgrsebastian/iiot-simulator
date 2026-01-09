package simulator

import (
	"math"
	"math/rand"
	"time"
)

// TimeseriesGenerator generates realistic timeseries values for welding parameters
type TimeseriesGenerator struct {
	rng *rand.Rand

	// Target values (setpoints)
	TargetCurrent       float64 // Amps
	TargetVoltage       float64 // Volts
	TargetWireFeedSpeed float64 // m/min
	TargetGasFlow       float64 // l/min
	TargetTravelSpeed   float64 // mm/s

	// State tracking for colored noise
	coloredNoiseState float64
	lastCurrent       float64
	lastVoltage       float64

	// Position simulation
	weldPathProgress float64
	weldPathLength   float64 // mm
}

// NewTimeseriesGenerator creates a new timeseries generator with default welding parameters
func NewTimeseriesGenerator() *TimeseriesGenerator {
	return &TimeseriesGenerator{
		rng: rand.New(rand.NewSource(time.Now().UnixNano())),

		// Default values for mild steel, 0.035-0.045" wire
		TargetCurrent:       200.0, // Amps
		TargetVoltage:       24.0,  // Volts
		TargetWireFeedSpeed: 9.6,   // m/min (~380 IPM)
		TargetGasFlow:       15.0,  // l/min (~32 CFH)
		TargetTravelSpeed:   10.0,  // mm/s

		weldPathLength: 500.0, // mm total weld path
	}
}

// Generate generates timeseries data based on current state and phase
func (tg *TimeseriesGenerator) Generate(state MachineState, phase WeldPhase, phaseProgress float64) TimeseriesData {
	data := TimeseriesData{
		State:     state,
		Timestamp: time.Now(),
	}

	switch state {
	case StateRunning:
		tg.generateRunningValues(&data, phase, phaseProgress)
	case StateSetup:
		tg.generateSetupValues(&data)
	default:
		tg.generateIdleValues(&data)
	}

	return data
}

func (tg *TimeseriesGenerator) generateRunningValues(data *TimeseriesData, phase WeldPhase, phaseProgress float64) {
	// Calculate phase multiplier for ramp-up/ramp-down
	var phaseMult float64
	var noiseLevel float64

	switch phase {
	case PhaseRampUp:
		// Exponential ramp-up: 1 - e^(-t/tau)
		tau := 0.15 // Time constant in seconds (normalized to phase progress)
		t := phaseProgress * 0.5 // Assume 0.5s ramp-up
		phaseMult = 1 - math.Exp(-t/tau)
		noiseLevel = 0.05 // 5% noise during ramp-up

	case PhaseSteady:
		phaseMult = 1.0
		noiseLevel = 0.02 // 2% noise in steady state

	case PhaseRampDown:
		// Exponential ramp-down: e^(-t/tau)
		tau := 0.15
		t := phaseProgress * 0.5 // Assume 0.5s ramp-down
		phaseMult = math.Exp(-t / tau)
		noiseLevel = 0.05 // 5% noise during ramp-down

	default:
		phaseMult = 0
		noiseLevel = 0
	}

	// Generate correlated current and voltage with noise
	commonFactor := tg.rng.NormFloat64() * 0.02 // Shared variance for correlation

	// Current with Gaussian noise and occasional spikes
	currentNoise := commonFactor + tg.rng.NormFloat64()*noiseLevel
	data.WeldingCurrent = tg.TargetCurrent * phaseMult * (1 + currentNoise)

	// Add occasional spikes (0.3% chance)
	if tg.rng.Float64() < 0.003 {
		spike := (tg.rng.Float64() - 0.5) * tg.TargetCurrent * 0.10
		data.WeldingCurrent += spike
	}

	// Ensure non-negative
	if data.WeldingCurrent < 0 {
		data.WeldingCurrent = 0
	}

	// Voltage correlated with current (0.4-0.6 correlation)
	voltageNoise := commonFactor*0.5 + tg.rng.NormFloat64()*(noiseLevel*0.5)
	data.Voltage = tg.TargetVoltage * phaseMult * (1 + voltageNoise)
	if data.Voltage < 0 {
		data.Voltage = 0
	}

	// Wire feed speed - very stable during running
	wireNoise := tg.rng.NormFloat64() * 0.005 // Only 0.5% noise
	data.WireFeedSpeed = tg.TargetWireFeedSpeed * phaseMult * (1 + wireNoise)
	if data.WireFeedSpeed < 0 {
		data.WireFeedSpeed = 0
	}

	// Gas flow - essentially constant, minimal noise
	gasNoise := tg.rng.NormFloat64() * 0.003 // 0.3% noise
	data.GasFlow = tg.TargetGasFlow * (1 + gasNoise)
	// Gas keeps flowing during ramp phases
	if phase == PhaseRampUp || phase == PhaseSteady || phase == PhaseRampDown {
		// Gas is on
	} else {
		data.GasFlow = 0
	}

	// Travel speed - follows weld path
	travelNoise := tg.rng.NormFloat64() * 0.02
	data.TravelSpeed = tg.TargetTravelSpeed * phaseMult * (1 + travelNoise)
	if data.TravelSpeed < 0 {
		data.TravelSpeed = 0
	}

	// Position simulation - follow a simple linear path
	tg.weldPathProgress += data.TravelSpeed * 0.001 // Progress per ms
	if tg.weldPathProgress > tg.weldPathLength {
		tg.weldPathProgress = 0 // Reset for next weld
	}

	// Simple X-Y motion along a line with some variation
	progress := tg.weldPathProgress / tg.weldPathLength
	data.PositionX = -250 + progress*500 + tg.rng.NormFloat64()*2
	data.PositionY = math.Sin(progress*math.Pi*4)*50 + tg.rng.NormFloat64()*2 // Slight wave pattern
	data.PositionZ = 200 + math.Sin(progress*math.Pi*2)*20 + tg.rng.NormFloat64()*1

	// Torch angle varies during weld
	data.TorchAngle = 30 + math.Sin(progress*math.Pi*2)*10 + tg.rng.NormFloat64()*2

	// Store for colored noise continuity
	tg.lastCurrent = data.WeldingCurrent
	tg.lastVoltage = data.Voltage
}

func (tg *TimeseriesGenerator) generateSetupValues(data *TimeseriesData) {
	// During setup, robot moves to position but no welding
	data.WeldingCurrent = 0
	data.Voltage = 0
	data.WireFeedSpeed = 0
	data.GasFlow = 0 // Gas off during setup
	data.TravelSpeed = 0

	// Robot at home position with slight movement
	data.PositionX = tg.rng.NormFloat64() * 5
	data.PositionY = tg.rng.NormFloat64() * 5
	data.PositionZ = 200 + tg.rng.NormFloat64()*2
	data.TorchAngle = 0

	// Reset weld path for next cycle
	tg.weldPathProgress = 0
}

func (tg *TimeseriesGenerator) generateIdleValues(data *TimeseriesData) {
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
	tg.TargetCurrent = current
	tg.TargetVoltage = voltage
	tg.TargetWireFeedSpeed = wireFeed
	tg.TargetGasFlow = gasFlow
	tg.TargetTravelSpeed = travelSpeed
}

// CalculatePhaseProgress returns the progress within the current weld phase (0-1)
func CalculatePhaseProgress(cycleStart time.Time, cycleTime time.Duration, phase WeldPhase) float64 {
	elapsed := time.Since(cycleStart)
	rampUpDuration := time.Duration(float64(cycleTime) * 0.05)
	steadyDuration := time.Duration(float64(cycleTime) * 0.90)
	rampDownDuration := time.Duration(float64(cycleTime) * 0.05)

	switch phase {
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
