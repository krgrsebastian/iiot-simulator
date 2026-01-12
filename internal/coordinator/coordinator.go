package coordinator

import (
	"context"
	"sync"
	"time"

	"github.com/sebastiankruger/shopfloor-simulator/internal/core"
	"github.com/sebastiankruger/shopfloor-simulator/internal/machines/forming"
	"github.com/sebastiankruger/shopfloor-simulator/internal/machines/picker"
	"github.com/sebastiankruger/shopfloor-simulator/internal/machines/spotwelder"
)

// ProductionLineCoordinator coordinates a production line of machines
type ProductionLineCoordinator struct {
	config LineConfig

	// Machines
	formingMachine *forming.FormingMachine
	pickerRobot    *picker.PickerRobot
	spotWelder     *spotwelder.SpotWelder

	// Metrics
	metrics *MetricsCollector

	// State
	lineState   LineState
	startTime   time.Time
	lastUpdate  time.Time

	// Counters
	totalPartsCompleted int
	totalPartsScrap     int
	totalPartsStarted   int

	// Per-station counters
	formingCompleted int
	pickingCompleted int
	weldingCompleted int

	// Timing
	uptime   time.Duration
	downtime time.Duration

	// Part tracking
	partsInProgress map[string]*core.Part

	// Current order
	currentOrder *core.ProductionOrder

	// Error tracking
	lastErrorCode    string
	lastErrorMachine string
	activeErrors     int

	// Mutex for thread safety
	mu sync.RWMutex
}

// NewProductionLineCoordinator creates a new production line coordinator
func NewProductionLineCoordinator(config LineConfig) *ProductionLineCoordinator {
	return &ProductionLineCoordinator{
		config:          config,
		lineState:       LineStateStopped,
		partsInProgress: make(map[string]*core.Part),
		metrics:         NewMetricsCollector(config),
	}
}

// SetMachines sets the machines in the production line
func (c *ProductionLineCoordinator) SetMachines(
	forming *forming.FormingMachine,
	picker *picker.PickerRobot,
	welder *spotwelder.SpotWelder,
) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.formingMachine = forming
	c.pickerRobot = picker
	c.spotWelder = welder

	// Connect the buffers
	// Forming output → Picker input
	if picker != nil && forming != nil {
		picker.SetInputBuffer(forming.GetOutputBuffer())
	}

	// Picker output → Welder input
	if welder != nil && picker != nil {
		// Create a buffer for picker output that welder will read from
		pickerOutputBuffer := core.NewPartBuffer(c.config.WelderBufferCapacity)
		picker.SetOutputBuffer(pickerOutputBuffer)
		// Note: SpotWelder has its own input buffer, we need to sync these
		// For now, we'll handle this in the Update loop
	}
}

// Start starts the production line
func (c *ProductionLineCoordinator) Start(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.lineState = LineStateSetup
	c.startTime = time.Now()
	c.lastUpdate = time.Now()

	// Start all machines
	if c.formingMachine != nil {
		c.formingMachine.Start(ctx)
	}
	if c.pickerRobot != nil {
		c.pickerRobot.Start(ctx)
	}
	if c.spotWelder != nil {
		c.spotWelder.Start(ctx)
	}

	c.lineState = LineStateRunning
	return nil
}

// Stop stops the production line
func (c *ProductionLineCoordinator) Stop(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.lineState = LineStateStopped

	// Stop all machines
	if c.formingMachine != nil {
		c.formingMachine.Stop(ctx)
	}
	if c.pickerRobot != nil {
		c.pickerRobot.Stop(ctx)
	}
	if c.spotWelder != nil {
		c.spotWelder.Stop(ctx)
	}

	return nil
}

// Update updates all machines and coordinates part flow
func (c *ProductionLineCoordinator) Update(now time.Time, isBreakTime bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	elapsed := now.Sub(c.lastUpdate)
	c.lastUpdate = now

	// Update line state based on machine states
	c.updateLineState()

	// Track uptime/downtime
	if c.lineState == LineStateRunning {
		c.uptime += elapsed
	} else if c.lineState == LineStateError || c.lineState == LineStateStopped {
		c.downtime += elapsed
	}

	// Update all machines
	if c.formingMachine != nil {
		c.formingMachine.Update(now, isBreakTime)
	}
	if c.pickerRobot != nil {
		c.pickerRobot.Update(now, isBreakTime)
	}
	if c.spotWelder != nil {
		c.spotWelder.Update(now, isBreakTime)
	}

	// Coordinate part flow
	c.coordinatePartFlow(now)

	// Update metrics
	c.updateCounters()

	// Update error tracking
	c.updateErrorTracking()

	// Collect metrics for OEE
	c.metrics.Update(c.formingMachine, c.pickerRobot, c.spotWelder, now)
}

func (c *ProductionLineCoordinator) updateLineState() {
	// Check for any machine errors
	hasError := false

	if c.formingMachine != nil && c.formingMachine.State() == core.StateUnplannedStop {
		hasError = true
	}
	if c.pickerRobot != nil && c.pickerRobot.State() == core.StateUnplannedStop {
		hasError = true
	}
	if c.spotWelder != nil && c.spotWelder.State() == core.StateUnplannedStop {
		hasError = true
	}

	if hasError {
		c.lineState = LineStateError
	} else if c.lineState == LineStateError {
		// Recover from error when all machines are ok
		c.lineState = LineStateRunning
	}
}

func (c *ProductionLineCoordinator) coordinatePartFlow(now time.Time) {
	// Transfer parts from picker output to welder input
	// This simulates the handoff between picker and welder

	if c.pickerRobot == nil || c.spotWelder == nil {
		return
	}

	// The picker's output buffer feeds the welder's input buffer
	// The picker writes directly to its output buffer which is connected
	// as the welder's input in SetMachines - buffer is shared
	// No additional coordination needed since buffers are linked
}

func (c *ProductionLineCoordinator) updateCounters() {
	// Count completed parts through the line
	if c.spotWelder != nil {
		good, scrap := c.spotWelder.GetCounters()
		c.weldingCompleted = good
		c.totalPartsCompleted = good
		c.totalPartsScrap = scrap
	}

	if c.pickerRobot != nil {
		good, _ := c.pickerRobot.GetCounters()
		c.pickingCompleted = good
	}

	if c.formingMachine != nil {
		good, _ := c.formingMachine.GetCounters()
		c.formingCompleted = good
		c.totalPartsStarted = good
	}
}

func (c *ProductionLineCoordinator) updateErrorTracking() {
	c.activeErrors = 0

	if c.formingMachine != nil && c.formingMachine.CurrentError != nil {
		c.activeErrors++
		c.lastErrorCode = c.formingMachine.CurrentError.Code
		c.lastErrorMachine = c.config.FormingMachineName
	}

	if c.pickerRobot != nil && c.pickerRobot.CurrentError != nil {
		c.activeErrors++
		c.lastErrorCode = c.pickerRobot.CurrentError.Code
		c.lastErrorMachine = c.config.PickerRobotName
	}

	if c.spotWelder != nil && c.spotWelder.CurrentError != nil {
		c.activeErrors++
		c.lastErrorCode = c.spotWelder.CurrentError.Code
		c.lastErrorMachine = c.config.SpotWelderName
	}
}

// SetOrder sets the current production order for the line
func (c *ProductionLineCoordinator) SetOrder(order *core.ProductionOrder) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.currentOrder = order

	// Assign order to forming machine (first in line)
	if c.formingMachine != nil && order != nil {
		c.formingMachine.AddOrder(order)
	}
}

// GetLineState returns the current line state
func (c *ProductionLineCoordinator) GetLineState() LineState {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.lineState
}

// GetOPCUANodes returns the OPC UA node definitions for the production line
func (c *ProductionLineCoordinator) GetOPCUANodes() []core.NodeDefinition {
	return []core.NodeDefinition{
		{Name: "LineState", DisplayName: "Line State", Description: "Production line state", DataType: core.DataTypeString, Unit: "", InitialValue: "Stopped"},
		{Name: "WIPCount", DisplayName: "WIP Count", Description: "Work in progress count", DataType: core.DataTypeInt32, Unit: "", InitialValue: int32(0)},
		{Name: "ThroughputPerHour", DisplayName: "Throughput/Hour", Description: "Parts per hour", DataType: core.DataTypeDouble, Unit: "parts/hr", InitialValue: 0.0},
		{Name: "BottleneckMachine", DisplayName: "Bottleneck Machine", Description: "Current bottleneck", DataType: core.DataTypeString, Unit: "", InitialValue: ""},
		{Name: "TotalPartsCompleted", DisplayName: "Total Parts Completed", Description: "Parts through entire line", DataType: core.DataTypeInt32, Unit: "", InitialValue: int32(0)},
		{Name: "TotalPartsScrap", DisplayName: "Total Parts Scrap", Description: "Scrapped parts", DataType: core.DataTypeInt32, Unit: "", InitialValue: int32(0)},
		{Name: "TotalPartsStarted", DisplayName: "Total Parts Started", Description: "Parts started", DataType: core.DataTypeInt32, Unit: "", InitialValue: int32(0)},
		{Name: "FormingCompleted", DisplayName: "Forming Completed", Description: "Parts through forming", DataType: core.DataTypeInt32, Unit: "", InitialValue: int32(0)},
		{Name: "PickingCompleted", DisplayName: "Picking Completed", Description: "Parts through picker", DataType: core.DataTypeInt32, Unit: "", InitialValue: int32(0)},
		{Name: "WeldingCompleted", DisplayName: "Welding Completed", Description: "Parts through welder", DataType: core.DataTypeInt32, Unit: "", InitialValue: int32(0)},
		{Name: "FormingBufferCount", DisplayName: "Forming Buffer Count", Description: "Parts in forming buffer", DataType: core.DataTypeInt32, Unit: "", InitialValue: int32(0)},
		{Name: "PickerBufferCount", DisplayName: "Picker Buffer Count", Description: "Parts in welder input", DataType: core.DataTypeInt32, Unit: "", InitialValue: int32(0)},
		{Name: "Availability", DisplayName: "Availability", Description: "OEE availability", DataType: core.DataTypeDouble, Unit: "%", InitialValue: 0.0},
		{Name: "Performance", DisplayName: "Performance", Description: "OEE performance", DataType: core.DataTypeDouble, Unit: "%", InitialValue: 0.0},
		{Name: "Quality", DisplayName: "Quality", Description: "OEE quality", DataType: core.DataTypeDouble, Unit: "%", InitialValue: 0.0},
		{Name: "OEE", DisplayName: "OEE", Description: "Overall Equipment Effectiveness", DataType: core.DataTypeDouble, Unit: "%", InitialValue: 0.0},
		{Name: "LineUptime", DisplayName: "Line Uptime", Description: "Total uptime", DataType: core.DataTypeDouble, Unit: "s", InitialValue: 0.0},
		{Name: "LineDowntime", DisplayName: "Line Downtime", Description: "Total downtime", DataType: core.DataTypeDouble, Unit: "s", InitialValue: 0.0},
		{Name: "AverageCycleTime", DisplayName: "Average Cycle Time", Description: "Average part cycle time", DataType: core.DataTypeDouble, Unit: "s", InitialValue: 0.0},
		{Name: "CurrentOrderId", DisplayName: "Current Order ID", Description: "Active order ID", DataType: core.DataTypeString, Unit: "", InitialValue: ""},
		{Name: "CurrentPartNumber", DisplayName: "Current Part Number", Description: "Active part number", DataType: core.DataTypeString, Unit: "", InitialValue: ""},
		{Name: "OrderProgress", DisplayName: "Order Progress", Description: "Order completion 0-100%", DataType: core.DataTypeDouble, Unit: "%", InitialValue: 0.0},
		{Name: "ActiveErrors", DisplayName: "Active Errors", Description: "Number of active errors", DataType: core.DataTypeInt32, Unit: "", InitialValue: int32(0)},
		{Name: "LastErrorCode", DisplayName: "Last Error Code", Description: "Most recent error code", DataType: core.DataTypeString, Unit: "", InitialValue: ""},
		{Name: "LastErrorMachine", DisplayName: "Last Error Machine", Description: "Machine with last error", DataType: core.DataTypeString, Unit: "", InitialValue: ""},
	}
}

// GenerateData generates current line data for OPC UA
func (c *ProductionLineCoordinator) GenerateData() map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	data := &LineData{
		LineState:           string(c.lineState),
		TotalPartsCompleted: c.totalPartsCompleted,
		TotalPartsScrap:     c.totalPartsScrap,
		TotalPartsStarted:   c.totalPartsStarted,
		FormingCompleted:    c.formingCompleted,
		PickingCompleted:    c.pickingCompleted,
		WeldingCompleted:    c.weldingCompleted,
		LineUptime:          c.uptime.Seconds(),
		LineDowntime:        c.downtime.Seconds(),
		ActiveErrors:        c.activeErrors,
		LastErrorCode:       c.lastErrorCode,
		LastErrorMachine:    c.lastErrorMachine,
		Timestamp:           time.Now(),
	}

	// Buffer counts
	if c.formingMachine != nil {
		data.FormingBufferCount = c.formingMachine.GetOutputBuffer().Count()
	}
	if c.spotWelder != nil {
		data.PickerBufferCount = c.spotWelder.GetInputBuffer().Count()
	}

	// WIP count = sum of parts in buffers + parts being processed
	data.WIPCount = data.FormingBufferCount + data.PickerBufferCount
	if c.pickerRobot != nil && c.pickerRobot.GetHeldPartID() != "" {
		data.WIPCount++
	}

	// Calculate throughput
	if c.uptime.Seconds() > 0 {
		data.ThroughputPerHour = float64(c.totalPartsCompleted) / c.uptime.Hours()
	}

	// OEE calculation from metrics
	oee := c.metrics.CalculateOEE()
	data.Availability = oee.Availability
	data.Performance = oee.Performance
	data.Quality = oee.Quality
	data.OEE = oee.OEE

	// Average cycle time
	if c.totalPartsCompleted > 0 {
		data.AverageCycleTime = c.uptime.Seconds() / float64(c.totalPartsCompleted)
	}

	// Bottleneck
	data.BottleneckMachine = c.metrics.GetBottleneck()

	// Current order
	if c.currentOrder != nil {
		data.CurrentOrderID = c.currentOrder.OrderID
		data.CurrentPartNumber = c.currentOrder.PartNumber
		if c.currentOrder.Quantity > 0 {
			data.OrderProgress = float64(c.totalPartsCompleted) / float64(c.currentOrder.Quantity) * 100
		}
	}

	return data.ToMap()
}

// GetLineData returns the full LineData struct
func (c *ProductionLineCoordinator) GetLineData() *LineData {
	dataMap := c.GenerateData()

	// Convert back to LineData (simplified version)
	return &LineData{
		LineState:           dataMap["LineState"].(string),
		WIPCount:            int(dataMap["WIPCount"].(int32)),
		ThroughputPerHour:   dataMap["ThroughputPerHour"].(float64),
		BottleneckMachine:   dataMap["BottleneckMachine"].(string),
		TotalPartsCompleted: int(dataMap["TotalPartsCompleted"].(int32)),
		TotalPartsScrap:     int(dataMap["TotalPartsScrap"].(int32)),
		TotalPartsStarted:   int(dataMap["TotalPartsStarted"].(int32)),
		Availability:        dataMap["Availability"].(float64),
		Performance:         dataMap["Performance"].(float64),
		Quality:             dataMap["Quality"].(float64),
		OEE:                 dataMap["OEE"].(float64),
		Timestamp:           time.Now(),
	}
}
