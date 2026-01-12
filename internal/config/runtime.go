package config

import (
	"fmt"
	"sync"
	"time"
)

// RuntimeConfig holds configuration values that can be changed at runtime.
// All methods are thread-safe.
type RuntimeConfig struct {
	mu             sync.RWMutex
	cycleTimeScale float64       // Multiplier: 0.1 - 10.0 (default 1.0)
	scrapRate      float64       // 0.0 - 0.5 (default from env)
	errorRate      float64       // 0.0 - 0.2 (default from env)
	baseCycleTime  time.Duration // Original cycle time from env
	baseSetupTime  time.Duration // Original setup time from env
}

// NewRuntimeConfig creates a new RuntimeConfig from the static Config.
func NewRuntimeConfig(cfg *Config) *RuntimeConfig {
	return &RuntimeConfig{
		cycleTimeScale: 1.0,
		scrapRate:      cfg.ScrapRate,
		errorRate:      cfg.ErrorRate,
		baseCycleTime:  cfg.CycleTime,
		baseSetupTime:  cfg.SetupTime,
	}
}

// GetCycleTimeScale returns the current cycle time multiplier.
func (rc *RuntimeConfig) GetCycleTimeScale() float64 {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	return rc.cycleTimeScale
}

// GetBaseCycleTime returns the original cycle time from env.
func (rc *RuntimeConfig) GetBaseCycleTime() time.Duration {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	return rc.baseCycleTime
}

// GetEffectiveCycleTime returns the cycle time adjusted by the scale factor.
// Higher scale = faster cycles (shorter duration).
func (rc *RuntimeConfig) GetEffectiveCycleTime() time.Duration {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	return time.Duration(float64(rc.baseCycleTime) / rc.cycleTimeScale)
}

// GetEffectiveSetupTime returns the setup time adjusted by the scale factor.
func (rc *RuntimeConfig) GetEffectiveSetupTime() time.Duration {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	return time.Duration(float64(rc.baseSetupTime) / rc.cycleTimeScale)
}

// GetEffectiveErrorDuration scales an error duration by the cycle time scale.
// Higher scale = faster simulation = shorter error duration.
func (rc *RuntimeConfig) GetEffectiveErrorDuration(baseDuration time.Duration) time.Duration {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	return time.Duration(float64(baseDuration) / rc.cycleTimeScale)
}

// GetScrapRate returns the current scrap rate (0.0 - 0.5).
func (rc *RuntimeConfig) GetScrapRate() float64 {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	return rc.scrapRate
}

// GetErrorRate returns the current error rate (0.0 - 0.2).
func (rc *RuntimeConfig) GetErrorRate() float64 {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	return rc.errorRate
}

// SetCycleTimeScale sets the cycle time multiplier.
// Valid range: 0.1 - 10.0
func (rc *RuntimeConfig) SetCycleTimeScale(scale float64) error {
	if scale < 0.1 || scale > 10.0 {
		return fmt.Errorf("cycle time scale must be between 0.1 and 10.0, got %f", scale)
	}
	rc.mu.Lock()
	defer rc.mu.Unlock()
	rc.cycleTimeScale = scale
	return nil
}

// SetScrapRate sets the scrap rate.
// Valid range: 0.0 - 0.5 (0% - 50%)
func (rc *RuntimeConfig) SetScrapRate(rate float64) error {
	if rate < 0.0 || rate > 0.5 {
		return fmt.Errorf("scrap rate must be between 0.0 and 0.5, got %f", rate)
	}
	rc.mu.Lock()
	defer rc.mu.Unlock()
	rc.scrapRate = rate
	return nil
}

// SetErrorRate sets the error rate.
// Valid range: 0.0 - 0.2 (0% - 20%)
func (rc *RuntimeConfig) SetErrorRate(rate float64) error {
	if rate < 0.0 || rate > 0.2 {
		return fmt.Errorf("error rate must be between 0.0 and 0.2, got %f", rate)
	}
	rc.mu.Lock()
	defer rc.mu.Unlock()
	rc.errorRate = rate
	return nil
}

// Snapshot returns a copy of all current values for safe reading.
type RuntimeConfigSnapshot struct {
	CycleTimeScale     float64
	BaseCycleTime      time.Duration
	EffectiveCycleTime time.Duration
	BaseSetupTime      time.Duration
	EffectiveSetupTime time.Duration
	ScrapRate          float64
	ErrorRate          float64
}

// Snapshot returns a point-in-time copy of all runtime config values.
func (rc *RuntimeConfig) Snapshot() RuntimeConfigSnapshot {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	return RuntimeConfigSnapshot{
		CycleTimeScale:     rc.cycleTimeScale,
		BaseCycleTime:      rc.baseCycleTime,
		EffectiveCycleTime: time.Duration(float64(rc.baseCycleTime) / rc.cycleTimeScale),
		BaseSetupTime:      rc.baseSetupTime,
		EffectiveSetupTime: time.Duration(float64(rc.baseSetupTime) / rc.cycleTimeScale),
		ScrapRate:          rc.scrapRate,
		ErrorRate:          rc.errorRate,
	}
}
