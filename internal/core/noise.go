package core

import (
	"math"
	"math/rand"
	"time"
)

// NoiseGenerator provides utilities for generating realistic sensor noise
type NoiseGenerator struct {
	rng *rand.Rand

	// State for colored/correlated noise
	coloredNoiseState map[string]float64
	lastValues        map[string]float64
}

// NewNoiseGenerator creates a new noise generator
func NewNoiseGenerator() *NoiseGenerator {
	return &NoiseGenerator{
		rng:               rand.New(rand.NewSource(time.Now().UnixNano())),
		coloredNoiseState: make(map[string]float64),
		lastValues:        make(map[string]float64),
	}
}

// Gaussian returns a value from a Gaussian distribution with given mean and stdDev
func (ng *NoiseGenerator) Gaussian(mean, stdDev float64) float64 {
	return mean + ng.rng.NormFloat64()*stdDev
}

// GaussianNoise returns Gaussian noise scaled by a percentage of the target value
func (ng *NoiseGenerator) GaussianNoise(target, noisePercent float64) float64 {
	noise := ng.rng.NormFloat64() * (target * noisePercent)
	return target + noise
}

// GaussianNoiseWithClamp returns Gaussian noise clamped to min/max values
func (ng *NoiseGenerator) GaussianNoiseWithClamp(target, noisePercent, min, max float64) float64 {
	value := ng.GaussianNoise(target, noisePercent)
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

// CorrelatedNoise generates noise correlated with a common factor
// Use this when two parameters should have some correlation (e.g., current and voltage)
func (ng *NoiseGenerator) CorrelatedNoise(target, noisePercent, commonFactor, correlationStrength float64) float64 {
	// Blend independent noise with common factor
	independentNoise := ng.rng.NormFloat64() * noisePercent * target
	correlatedComponent := commonFactor * correlationStrength * target
	return target + independentNoise + correlatedComponent
}

// CommonFactor generates a shared noise factor for correlation between parameters
func (ng *NoiseGenerator) CommonFactor(scale float64) float64 {
	return ng.rng.NormFloat64() * scale
}

// ColoredNoise generates noise with temporal correlation (smooth transitions)
// alpha: smoothing factor (0 = pure white noise, 1 = constant value)
func (ng *NoiseGenerator) ColoredNoise(key string, target, noisePercent, alpha float64) float64 {
	// Get previous state
	prevState, exists := ng.coloredNoiseState[key]
	if !exists {
		prevState = 0
	}

	// Generate new noise
	whiteNoise := ng.rng.NormFloat64() * noisePercent * target

	// Apply exponential smoothing
	newState := alpha*prevState + (1-alpha)*whiteNoise

	// Store state
	ng.coloredNoiseState[key] = newState

	return target + newState
}

// Spike generates occasional spikes in the signal
// probability: chance of spike occurring (0-1)
// maxMagnitude: maximum spike magnitude as percentage of target
func (ng *NoiseGenerator) Spike(target, probability, maxMagnitude float64) float64 {
	if ng.rng.Float64() < probability {
		spike := (ng.rng.Float64() - 0.5) * 2 * target * maxMagnitude
		return spike
	}
	return 0
}

// RampValue calculates a value during ramp-up or ramp-down
// progress: 0-1 where 0 is start and 1 is end of ramp
// rampUp: true for ramp-up, false for ramp-down
// tau: time constant for exponential curve (smaller = faster ramp)
func (ng *NoiseGenerator) RampValue(target float64, progress float64, rampUp bool, tau float64) float64 {
	// Normalize progress to typical ramp duration
	t := progress * 0.5 // Assume 0.5s ramp duration normalized

	var multiplier float64
	if rampUp {
		// Exponential ramp-up: 1 - e^(-t/tau)
		multiplier = 1 - math.Exp(-t/tau)
	} else {
		// Exponential ramp-down: e^(-t/tau)
		multiplier = math.Exp(-t / tau)
	}

	return target * multiplier
}

// Uniform returns a uniform random value in [min, max]
func (ng *NoiseGenerator) Uniform(min, max float64) float64 {
	return min + ng.rng.Float64()*(max-min)
}

// UniformInt returns a uniform random integer in [min, max]
func (ng *NoiseGenerator) UniformInt(min, max int) int {
	return min + ng.rng.Intn(max-min+1)
}

// Bool returns true with the given probability
func (ng *NoiseGenerator) Bool(probability float64) bool {
	return ng.rng.Float64() < probability
}

// ShouldTrigger returns true with probability scaled by tickDuration vs cycleDuration
// Use this for per-tick probability checks (e.g., error probability per cycle)
func (ng *NoiseGenerator) ShouldTrigger(probabilityPerCycle float64, tickDuration, cycleDuration time.Duration) bool {
	// Scale probability for tick frequency
	scaledProbability := probabilityPerCycle * float64(tickDuration) / float64(cycleDuration)
	return ng.rng.Float64() < scaledProbability
}

// SelectWeighted selects from a slice of weights, returning the index
func (ng *NoiseGenerator) SelectWeighted(weights []float64) int {
	total := 0.0
	for _, w := range weights {
		total += w
	}

	r := ng.rng.Float64() * total
	cumulative := 0.0
	for i, w := range weights {
		cumulative += w
		if r <= cumulative {
			return i
		}
	}
	return len(weights) - 1
}

// SinusoidalVariation adds sinusoidal variation to a value
// amplitude: variation magnitude as percentage of target
// progress: 0-1 progress through the variation period
func (ng *NoiseGenerator) SinusoidalVariation(target, amplitude, progress float64) float64 {
	return target + math.Sin(progress*2*math.Pi)*target*amplitude
}

// DriftValue generates a slowly drifting value (simulates sensor drift)
// driftRate: maximum drift per tick as percentage
func (ng *NoiseGenerator) DriftValue(key string, target, driftRate float64) float64 {
	last, exists := ng.lastValues[key]
	if !exists {
		last = target
	}

	// Random walk with mean reversion
	drift := ng.rng.NormFloat64() * driftRate * target
	meanReversion := (target - last) * 0.01 // Slowly return to target

	newValue := last + drift + meanReversion
	ng.lastValues[key] = newValue

	return newValue
}

// ClampPositive ensures a value is non-negative
func ClampPositive(value float64) float64 {
	if value < 0 {
		return 0
	}
	return value
}

// Clamp ensures a value is within bounds
func Clamp(value, min, max float64) float64 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}
