package coordinator

import (
	"sync"
	"time"

	"github.com/sebastiankruger/shopfloor-simulator/internal/core"
	"github.com/sebastiankruger/shopfloor-simulator/internal/machines/forming"
	"github.com/sebastiankruger/shopfloor-simulator/internal/machines/picker"
	"github.com/sebastiankruger/shopfloor-simulator/internal/machines/spotwelder"
)

// OEEResult contains calculated OEE values
type OEEResult struct {
	Availability float64 // Uptime / Total time (0-100%)
	Performance  float64 // Actual / Theoretical output (0-100%)
	Quality      float64 // Good parts / Total parts (0-100%)
	OEE          float64 // A * P * Q (0-100%)
}

// MetricsCollector collects and calculates production line metrics
type MetricsCollector struct {
	config LineConfig

	// Time tracking
	startTime    time.Time
	lastUpdate   time.Time
	totalRunning time.Duration
	totalStopped time.Duration

	// Production counts
	theoreticalOutput int
	actualOutput      int
	goodParts         int
	totalParts        int

	// Per-machine metrics
	formingMetrics MachineMetrics
	pickerMetrics  MachineMetrics
	welderMetrics  MachineMetrics

	// Cycle time tracking for bottleneck detection
	formingCycleTimes []time.Duration
	pickerCycleTimes  []time.Duration
	welderCycleTimes  []time.Duration

	// Last known states
	lastFormingCount int
	lastPickerCount  int
	lastWelderCount  int

	// Cycle time calculation
	lastFormingTime time.Time
	lastPickerTime  time.Time
	lastWelderTime  time.Time

	mu sync.RWMutex
}

// NewMetricsCollector creates a new metrics collector
func NewMetricsCollector(config LineConfig) *MetricsCollector {
	return &MetricsCollector{
		config:            config,
		startTime:         time.Now(),
		lastUpdate:        time.Now(),
		formingCycleTimes: make([]time.Duration, 0, 100),
		pickerCycleTimes:  make([]time.Duration, 0, 100),
		welderCycleTimes:  make([]time.Duration, 0, 100),
		lastFormingTime:   time.Now(),
		lastPickerTime:    time.Now(),
		lastWelderTime:    time.Now(),
	}
}

// Update updates metrics from current machine states
func (m *MetricsCollector) Update(
	forming *forming.FormingMachine,
	picker *picker.PickerRobot,
	welder *spotwelder.SpotWelder,
	now time.Time,
) {
	m.mu.Lock()
	defer m.mu.Unlock()

	elapsed := now.Sub(m.lastUpdate)
	m.lastUpdate = now

	// Update machine metrics
	if forming != nil {
		m.updateMachineMetrics(&m.formingMetrics, m.config.FormingMachineName,
			forming.State(), forming.GetCycleCount(), forming.GoodParts, forming.ScrapParts, elapsed)

		// Track cycle times
		currentCount := forming.GetCycleCount()
		if currentCount > m.lastFormingCount {
			cycleTime := now.Sub(m.lastFormingTime)
			m.addCycleTime(&m.formingCycleTimes, cycleTime)
			m.lastFormingTime = now
			m.lastFormingCount = currentCount
		}
	}

	if picker != nil {
		good, scrap := picker.GetCounters()
		m.updateMachineMetrics(&m.pickerMetrics, m.config.PickerRobotName,
			picker.State(), picker.GetCycleCount(), good, scrap, elapsed)

		currentCount := picker.GetCycleCount()
		if currentCount > m.lastPickerCount {
			cycleTime := now.Sub(m.lastPickerTime)
			m.addCycleTime(&m.pickerCycleTimes, cycleTime)
			m.lastPickerTime = now
			m.lastPickerCount = currentCount
		}
	}

	if welder != nil {
		good, scrap := welder.GetCounters()
		m.updateMachineMetrics(&m.welderMetrics, m.config.SpotWelderName,
			welder.State(), welder.GetCycleCount(), good, scrap, elapsed)

		currentCount := welder.GetCycleCount()
		if currentCount > m.lastWelderCount {
			cycleTime := now.Sub(m.lastWelderTime)
			m.addCycleTime(&m.welderCycleTimes, cycleTime)
			m.lastWelderTime = now
			m.lastWelderCount = currentCount
		}
	}

	// Update production counts (based on output of final station)
	if welder != nil {
		good, scrap := welder.GetCounters()
		m.actualOutput = good
		m.goodParts = good
		m.totalParts = good + scrap
	}

	// Calculate theoretical output based on elapsed time
	totalElapsed := now.Sub(m.startTime)
	theoreticalCycles := int(totalElapsed / m.config.TheoreticalCycleTime)
	m.theoreticalOutput = theoreticalCycles

	// Track running vs stopped time
	allRunning := true
	if forming != nil && forming.State() != core.StateRunning {
		allRunning = false
	}
	if picker != nil && picker.State() != core.StateRunning {
		allRunning = false
	}
	if welder != nil && welder.State() != core.StateRunning {
		allRunning = false
	}

	if allRunning {
		m.totalRunning += elapsed
	} else {
		m.totalStopped += elapsed
	}
}

func (m *MetricsCollector) updateMachineMetrics(
	metrics *MachineMetrics,
	name string,
	state core.MachineState,
	cycleCount, goodParts, scrapParts int,
	elapsed time.Duration,
) {
	metrics.Name = name
	metrics.State = state
	metrics.CycleCount = cycleCount
	metrics.GoodParts = goodParts
	metrics.ScrapParts = scrapParts

	if state == core.StateRunning {
		metrics.TotalUptime += elapsed
	} else if state == core.StateUnplannedStop {
		metrics.TotalDowntime += elapsed
	}
}

func (m *MetricsCollector) addCycleTime(times *[]time.Duration, cycleTime time.Duration) {
	*times = append(*times, cycleTime)

	// Keep only last 100 cycle times for moving average
	if len(*times) > 100 {
		*times = (*times)[1:]
	}
}

// CalculateOEE calculates overall equipment effectiveness
func (m *MetricsCollector) CalculateOEE() OEEResult {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := OEEResult{}

	// Availability = Running time / Total time
	totalTime := m.totalRunning + m.totalStopped
	if totalTime > 0 {
		result.Availability = float64(m.totalRunning) / float64(totalTime) * 100
	}

	// Performance = Actual output / Theoretical output
	if m.theoreticalOutput > 0 {
		result.Performance = float64(m.actualOutput) / float64(m.theoreticalOutput) * 100
		if result.Performance > 100 {
			result.Performance = 100 // Cap at 100%
		}
	}

	// Quality = Good parts / Total parts
	if m.totalParts > 0 {
		result.Quality = float64(m.goodParts) / float64(m.totalParts) * 100
	} else {
		result.Quality = 100 // No parts = 100% quality (nothing scrapped)
	}

	// OEE = Availability * Performance * Quality
	result.OEE = (result.Availability / 100) * (result.Performance / 100) * (result.Quality / 100) * 100

	return result
}

// GetBottleneck identifies the current bottleneck machine
func (m *MetricsCollector) GetBottleneck() string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Bottleneck is the machine with the longest average cycle time
	formingAvg := m.averageCycleTime(m.formingCycleTimes)
	pickerAvg := m.averageCycleTime(m.pickerCycleTimes)
	welderAvg := m.averageCycleTime(m.welderCycleTimes)

	// If no data yet, return empty
	if formingAvg == 0 && pickerAvg == 0 && welderAvg == 0 {
		return ""
	}

	// Find the longest cycle time
	maxTime := formingAvg
	bottleneck := m.config.FormingMachineName

	if pickerAvg > maxTime {
		maxTime = pickerAvg
		bottleneck = m.config.PickerRobotName
	}

	if welderAvg > maxTime {
		bottleneck = m.config.SpotWelderName
	}

	return bottleneck
}

func (m *MetricsCollector) averageCycleTime(times []time.Duration) time.Duration {
	if len(times) == 0 {
		return 0
	}

	var total time.Duration
	for _, t := range times {
		total += t
	}

	return total / time.Duration(len(times))
}

// GetMachineMetrics returns metrics for a specific machine
func (m *MetricsCollector) GetMachineMetrics(machineName string) MachineMetrics {
	m.mu.RLock()
	defer m.mu.RUnlock()

	switch machineName {
	case m.config.FormingMachineName:
		metrics := m.formingMetrics
		metrics.AverageCycleTime = m.averageCycleTime(m.formingCycleTimes)
		metrics.IsBottleneck = m.GetBottleneck() == machineName
		return metrics
	case m.config.PickerRobotName:
		metrics := m.pickerMetrics
		metrics.AverageCycleTime = m.averageCycleTime(m.pickerCycleTimes)
		metrics.IsBottleneck = m.GetBottleneck() == machineName
		return metrics
	case m.config.SpotWelderName:
		metrics := m.welderMetrics
		metrics.AverageCycleTime = m.averageCycleTime(m.welderCycleTimes)
		metrics.IsBottleneck = m.GetBottleneck() == machineName
		return metrics
	default:
		return MachineMetrics{}
	}
}

// GetAllMachineMetrics returns metrics for all machines
func (m *MetricsCollector) GetAllMachineMetrics() []MachineMetrics {
	return []MachineMetrics{
		m.GetMachineMetrics(m.config.FormingMachineName),
		m.GetMachineMetrics(m.config.PickerRobotName),
		m.GetMachineMetrics(m.config.SpotWelderName),
	}
}

// Reset resets all metrics
func (m *MetricsCollector) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.startTime = time.Now()
	m.lastUpdate = time.Now()
	m.totalRunning = 0
	m.totalStopped = 0
	m.theoreticalOutput = 0
	m.actualOutput = 0
	m.goodParts = 0
	m.totalParts = 0

	m.formingMetrics = MachineMetrics{}
	m.pickerMetrics = MachineMetrics{}
	m.welderMetrics = MachineMetrics{}

	m.formingCycleTimes = make([]time.Duration, 0, 100)
	m.pickerCycleTimes = make([]time.Duration, 0, 100)
	m.welderCycleTimes = make([]time.Duration, 0, 100)

	m.lastFormingCount = 0
	m.lastPickerCount = 0
	m.lastWelderCount = 0

	m.lastFormingTime = time.Now()
	m.lastPickerTime = time.Now()
	m.lastWelderTime = time.Now()
}
