package simulator

import (
	"time"

	"github.com/sebastiankruger/shopfloor-simulator/internal/machines/welding"
)

// TimeseriesGenerator wraps the welding timeseries generator for backward compatibility
type TimeseriesGenerator struct {
	gen *welding.TimeseriesGenerator

	// Legacy fields for compatibility
	TargetCurrent       float64 // Amps
	TargetVoltage       float64 // Volts
	TargetWireFeedSpeed float64 // m/min
	TargetGasFlow       float64 // l/min
	TargetTravelSpeed   float64 // mm/s
}

// NewTimeseriesGenerator creates a new timeseries generator with default welding parameters
func NewTimeseriesGenerator() *TimeseriesGenerator {
	cfg := welding.DefaultWeldingConfig()
	return &TimeseriesGenerator{
		gen: welding.NewTimeseriesGenerator(cfg),

		// Copy defaults for legacy access
		TargetCurrent:       cfg.TargetCurrent,
		TargetVoltage:       cfg.TargetVoltage,
		TargetWireFeedSpeed: cfg.TargetWireFeedSpeed,
		TargetGasFlow:       cfg.TargetGasFlow,
		TargetTravelSpeed:   cfg.TargetTravelSpeed,
	}
}

// Generate generates timeseries data based on current state and phase
func (tg *TimeseriesGenerator) Generate(state MachineState, phase WeldPhase, phaseProgress float64) TimeseriesData {
	// Generate using the welding generator
	data := tg.gen.Generate(state, phase, phaseProgress)

	// Convert to legacy TimeseriesData
	return TimeseriesData{
		WeldingCurrent:    data.WeldingCurrent,
		Voltage:           data.Voltage,
		WireFeedSpeed:     data.WireFeedSpeed,
		GasFlow:           data.GasFlow,
		TravelSpeed:       data.TravelSpeed,
		ArcTime:           data.ArcTime,
		PositionX:         data.PositionX,
		PositionY:         data.PositionY,
		PositionZ:         data.PositionZ,
		TorchAngle:        data.TorchAngle,
		State:             data.State,
		GoodParts:         data.GoodParts,
		ScrapParts:        data.ScrapParts,
		CurrentOrderID:    data.CurrentOrderID,
		CurrentPartNumber: data.CurrentPartNumber,
		CycleProgress:     data.CycleProgress,
		ErrorCode:         data.ErrorCode,
		ErrorMessage:      data.ErrorMessage,
		ErrorTimestamp:    data.ErrorTimestamp,
		Timestamp:         data.Timestamp,
	}
}

// SetTargets allows updating the target welding parameters
func (tg *TimeseriesGenerator) SetTargets(current, voltage, wireFeed, gasFlow, travelSpeed float64) {
	tg.gen.SetTargets(current, voltage, wireFeed, gasFlow, travelSpeed)

	// Update legacy fields
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
