package line

import (
	"context"
	"time"

	"github.com/sebastiankruger/shopfloor-simulator/internal/config"
	"github.com/sebastiankruger/shopfloor-simulator/internal/coordinator"
	"github.com/sebastiankruger/shopfloor-simulator/internal/core"
	"github.com/sebastiankruger/shopfloor-simulator/internal/machines/forming"
	"github.com/sebastiankruger/shopfloor-simulator/internal/machines/picker"
	"github.com/sebastiankruger/shopfloor-simulator/internal/machines/spotwelder"
	"github.com/sebastiankruger/shopfloor-simulator/internal/opcua"
)

// ProductionLineRunner manages a complete production line
type ProductionLineRunner struct {
	config        config.Config
	runtimeConfig *config.RuntimeConfig

	// Machines
	formingMachine *forming.FormingMachine
	pickerRobot    *picker.PickerRobot
	spotWelder     *spotwelder.SpotWelder

	// Coordinator
	coordinator *coordinator.ProductionLineCoordinator

	// OPC UA server
	opcuaServer *opcua.Server
}

// NewProductionLineRunner creates a new production line runner
func NewProductionLineRunner(cfg config.Config) (*ProductionLineRunner, error) {
	// Create runtime config for dynamic parameter adjustment
	runtimeCfg := config.NewRuntimeConfig(&cfg)

	plr := &ProductionLineRunner{
		config:        cfg,
		runtimeConfig: runtimeCfg,
	}

	// Create machine configurations with runtime config
	baseMachineConfig := core.MachineConfig{
		CycleTime:       cfg.CycleTime,
		SetupTime:       cfg.SetupTime,
		ErrorRate:       cfg.ErrorRate,
		ScrapRate:       cfg.ScrapRate,
		PublishInterval: cfg.PublishInterval,
		Runtime:         runtimeCfg,
	}

	// Create forming machine (ns=2)
	formingConfig := forming.DefaultFormingConfig()
	plr.formingMachine = forming.NewFormingMachine(
		"FormingMachine",
		baseMachineConfig,
		formingConfig,
	)

	// Create picker robot (ns=3) with faster cycle time
	pickerMachineConfig := baseMachineConfig
	pickerMachineConfig.CycleTime = cfg.CycleTime / 3 // Picker is faster
	// Note: Runtime is already set from baseMachineConfig
	pickerConfig := picker.DefaultPickerConfig()
	plr.pickerRobot = picker.NewPickerRobot(
		"PickerRobot",
		pickerMachineConfig,
		pickerConfig,
	)

	// Create spot welder (ns=4)
	welderConfig := spotwelder.DefaultSpotWelderConfig()
	plr.spotWelder = spotwelder.NewSpotWelder(
		"SpotWelder",
		baseMachineConfig,
		welderConfig,
	)

	// Create coordinator (ns=5)
	coordConfig := coordinator.DefaultLineConfig()
	coordConfig.LineName = cfg.SimulatorName
	plr.coordinator = coordinator.NewProductionLineCoordinator(coordConfig)

	// Connect machines to coordinator
	plr.coordinator.SetMachines(plr.formingMachine, plr.pickerRobot, plr.spotWelder)

	// Connect picker output to welder input
	plr.pickerRobot.SetOutputBuffer(plr.spotWelder.GetInputBuffer())

	// Connect picker input to forming output
	plr.pickerRobot.SetInputBuffer(plr.formingMachine.GetOutputBuffer())

	return plr, nil
}

// SetupOPCUA creates and configures the OPC UA server with multiple namespaces
func (plr *ProductionLineRunner) SetupOPCUA(port int, name string) error {
	var err error
	plr.opcuaServer, err = opcua.NewServer(port, name)
	if err != nil {
		return err
	}

	// Register namespace for Forming Machine (ns=2)
	if err := plr.opcuaServer.RegisterNamespace(
		core.NamespaceForming,
		"FormingMachine",
		"Sheet metal forming machine",
		plr.formingMachine.GetOPCUANodes(),
	); err != nil {
		return err
	}

	// Register namespace for Picker Robot (ns=3)
	if err := plr.opcuaServer.RegisterNamespace(
		core.NamespacePicker,
		"PickerRobot",
		"6-axis pick and place robot",
		plr.pickerRobot.GetOPCUANodes(),
	); err != nil {
		return err
	}

	// Register namespace for Spot Welder (ns=4)
	if err := plr.opcuaServer.RegisterNamespace(
		core.NamespaceSpotWelder,
		"SpotWelder",
		"Stud spot welding machine",
		plr.spotWelder.GetOPCUANodes(),
	); err != nil {
		return err
	}

	// Note: ns=5 (ProductionLine) removed - KPIs calculated externally

	return nil
}

// StartOPCUA starts the OPC UA server
func (plr *ProductionLineRunner) StartOPCUA(ctx context.Context) error {
	return plr.opcuaServer.Start(ctx)
}

// StopOPCUA stops the OPC UA server
func (plr *ProductionLineRunner) StopOPCUA(ctx context.Context) error {
	return plr.opcuaServer.Stop(ctx)
}

// Start starts the production line
func (plr *ProductionLineRunner) Start(ctx context.Context) error {
	return plr.coordinator.Start(ctx)
}

// Stop stops the production line
func (plr *ProductionLineRunner) Stop(ctx context.Context) error {
	return plr.coordinator.Stop(ctx)
}

// Update updates all machines and OPC UA values
func (plr *ProductionLineRunner) Update(now time.Time, isBreakTime bool) {
	// Update coordinator (which updates all machines)
	plr.coordinator.Update(now, isBreakTime)

	// Update OPC UA values for each namespace (ns=2, ns=3, ns=4 only)
	plr.opcuaServer.UpdateNamespaceValues(core.NamespaceForming, plr.formingMachine.GenerateData())
	plr.opcuaServer.UpdateNamespaceValues(core.NamespacePicker, plr.pickerRobot.GenerateData())
	plr.opcuaServer.UpdateNamespaceValues(core.NamespaceSpotWelder, plr.spotWelder.GenerateData())
}

// AddOrder adds an order to the production line
func (plr *ProductionLineRunner) AddOrder(order *core.ProductionOrder) {
	plr.coordinator.SetOrder(order)
}

// GetCoordinator returns the coordinator for external access
func (plr *ProductionLineRunner) GetCoordinator() *coordinator.ProductionLineCoordinator {
	return plr.coordinator
}

// GetFormingMachine returns the forming machine
func (plr *ProductionLineRunner) GetFormingMachine() *forming.FormingMachine {
	return plr.formingMachine
}

// GetPickerRobot returns the picker robot
func (plr *ProductionLineRunner) GetPickerRobot() *picker.PickerRobot {
	return plr.pickerRobot
}

// GetSpotWelder returns the spot welder
func (plr *ProductionLineRunner) GetSpotWelder() *spotwelder.SpotWelder {
	return plr.spotWelder
}

// GetLineState returns the current line state
func (plr *ProductionLineRunner) GetLineState() coordinator.LineState {
	return plr.coordinator.GetLineState()
}

// GetRuntimeConfig returns the runtime configuration for dynamic adjustments
func (plr *ProductionLineRunner) GetRuntimeConfig() *config.RuntimeConfig {
	return plr.runtimeConfig
}
