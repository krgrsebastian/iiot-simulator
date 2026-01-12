package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/sebastiankruger/shopfloor-simulator/internal/core"
	"github.com/sebastiankruger/shopfloor-simulator/internal/line"
	"github.com/sebastiankruger/shopfloor-simulator/internal/simulator"
)

// Handler handles REST API requests for the simulator
type Handler struct {
	simulatorMode string
	simulatorName string

	// For standalone welding robot mode
	stateMachine *simulator.StateMachine
	tsGenerator  *simulator.TimeseriesGenerator

	// For production line mode
	runner *line.ProductionLineRunner
}

// NewStandaloneHandler creates an API handler for standalone welding robot mode
func NewStandaloneHandler(name string, sm *simulator.StateMachine, ts *simulator.TimeseriesGenerator) *Handler {
	return &Handler{
		simulatorMode: "welding-robot",
		simulatorName: name,
		stateMachine:  sm,
		tsGenerator:   ts,
	}
}

// NewProductionLineHandler creates an API handler for production line mode
func NewProductionLineHandler(name string, runner *line.ProductionLineRunner) *Handler {
	return &Handler{
		simulatorMode: "production-line",
		simulatorName: name,
		runner:        runner,
	}
}

// HandleStatus handles GET /api/status
func (h *Handler) HandleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	resp := StatusResponse{
		Mode:          h.simulatorMode,
		SimulatorName: h.simulatorName,
	}

	if h.simulatorMode == "production-line" && h.runner != nil {
		resp.Machines = []MachineInfo{
			{ID: "forming", Name: "FormingMachine", Type: "Forming Press", Namespace: core.NamespaceForming},
			{ID: "picker", Name: "PickerRobot", Type: "Pick & Place Robot", Namespace: core.NamespacePicker},
			{ID: "spotwelder", Name: "SpotWelder", Type: "Spot Welding Machine", Namespace: core.NamespaceSpotWelder},
		}

		if order := h.runner.GetCoordinator().GetCurrentOrder(); order != nil {
			completed, _ := h.runner.GetCoordinator().GetOrderProgress()
			resp.CurrentOrder = &OrderInfo{
				OrderID:    order.OrderID,
				PartNumber: order.PartNumber,
				Quantity:   order.Quantity,
				Completed:  completed,
				Scrap:      order.QuantityScrap,
				Status:     order.Status,
			}
		}
	} else if h.stateMachine != nil {
		resp.Machines = []MachineInfo{
			{ID: "welding", Name: "WeldingRobot", Type: "Welding Robot", Namespace: core.NamespaceWelding},
		}

		if order := h.stateMachine.GetCurrentOrder(); order != nil {
			resp.CurrentOrder = &OrderInfo{
				OrderID:    order.OrderID,
				PartNumber: order.PartNumber,
				Quantity:   order.Quantity,
				Completed:  order.QuantityCompleted,
				Scrap:      order.QuantityScrap,
				Status:     string(order.Status),
			}
		}
	}

	h.writeJSON(w, resp)
}

// HandleMachines handles GET /api/machines
func (h *Handler) HandleMachines(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	resp := MachineListResponse{
		Machines: []MachineSummary{},
	}

	if h.simulatorMode == "production-line" && h.runner != nil {
		// Forming Machine
		fm := h.runner.GetFormingMachine()
		fmGood, fmScrap := fm.GetCounters()
		resp.Machines = append(resp.Machines, MachineSummary{
			ID:            "forming",
			Name:          fm.Name(),
			Type:          fm.MachineType(),
			Namespace:     core.NamespaceForming,
			State:         int(fm.State()),
			StateName:     fm.State().String(),
			GoodParts:     fmGood,
			ScrapParts:    fmScrap,
			CycleProgress: fm.GetCycleProgress(),
		})

		// Picker Robot
		pr := h.runner.GetPickerRobot()
		prGood, prScrap := pr.GetCounters()
		resp.Machines = append(resp.Machines, MachineSummary{
			ID:            "picker",
			Name:          pr.Name(),
			Type:          pr.MachineType(),
			Namespace:     core.NamespacePicker,
			State:         int(pr.State()),
			StateName:     pr.State().String(),
			GoodParts:     prGood,
			ScrapParts:    prScrap,
			CycleProgress: pr.GetCycleProgress(),
		})

		// Spot Welder
		sw := h.runner.GetSpotWelder()
		swGood, swScrap := sw.GetCounters()
		resp.Machines = append(resp.Machines, MachineSummary{
			ID:            "spotwelder",
			Name:          sw.Name(),
			Type:          sw.MachineType(),
			Namespace:     core.NamespaceSpotWelder,
			State:         int(sw.State()),
			StateName:     sw.State().String(),
			GoodParts:     swGood,
			ScrapParts:    swScrap,
			CycleProgress: sw.GetCycleProgress(),
		})
	} else if h.stateMachine != nil {
		state := h.stateMachine.GetState()
		goodParts, scrapParts, _ := h.stateMachine.GetCounters()
		resp.Machines = append(resp.Machines, MachineSummary{
			ID:            "welding",
			Name:          "WeldingRobot",
			Type:          "Welding Robot",
			Namespace:     core.NamespaceWelding,
			State:         int(state.State),
			StateName:     state.State.String(),
			GoodParts:     goodParts,
			ScrapParts:    scrapParts,
			CycleProgress: h.stateMachine.GetCycleProgress(),
		})
	}

	h.writeJSON(w, resp)
}

// HandleMachineDetail handles GET /api/machines/{id}
func (h *Handler) HandleMachineDetail(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract machine ID from path
	path := strings.TrimPrefix(r.URL.Path, "/api/machines/")
	machineID := strings.TrimSuffix(path, "/")

	if machineID == "" {
		http.Error(w, "Machine ID required", http.StatusBadRequest)
		return
	}

	var resp *MachineDetailResponse

	if h.simulatorMode == "production-line" && h.runner != nil {
		resp = h.getProductionLineMachineDetail(machineID)
	} else if h.stateMachine != nil {
		resp = h.getWeldingRobotDetail(machineID)
	}

	if resp == nil {
		http.Error(w, "Machine not found", http.StatusNotFound)
		return
	}

	h.writeJSON(w, resp)
}

func (h *Handler) getProductionLineMachineDetail(machineID string) *MachineDetailResponse {
	var machine core.MachineSimulator
	var namespace uint16

	switch machineID {
	case "forming":
		machine = h.runner.GetFormingMachine()
		namespace = core.NamespaceForming
	case "picker":
		machine = h.runner.GetPickerRobot()
		namespace = core.NamespacePicker
	case "spotwelder":
		machine = h.runner.GetSpotWelder()
		namespace = core.NamespaceSpotWelder
	default:
		return nil
	}

	nodes := []NodeInfo{}
	for _, nd := range machine.GetOPCUANodes() {
		nodeID := fmt.Sprintf("ns=%d;s=%s.%s", namespace, machine.Name(), nd.Name)
		nodes = append(nodes, NodeInfo{
			Name:        nd.Name,
			NodeID:      nodeID,
			DataType:    DataTypeToString(int(nd.DataType)),
			Unit:        nd.Unit,
			Description: nd.Description,
		})
	}

	return &MachineDetailResponse{
		ID:        machineID,
		Name:      machine.Name(),
		Type:      machine.MachineType(),
		Namespace: namespace,
		State:     int(machine.State()),
		StateName: machine.State().String(),
		Data:      machine.GenerateData(),
		Nodes:     nodes,
	}
}

func (h *Handler) getWeldingRobotDetail(machineID string) *MachineDetailResponse {
	if machineID != "welding" {
		return nil
	}

	state := h.stateMachine.GetState()
	goodParts, scrapParts, arcTime := h.stateMachine.GetCounters()

	// Generate current data
	var phaseProgress float64
	if state.State == simulator.StateRunning {
		phaseProgress = h.stateMachine.GetCycleProgress() / 100.0
	}
	tsData := h.tsGenerator.Generate(state.State, state.WeldPhase, phaseProgress)

	// Build data map
	data := map[string]interface{}{
		"WeldingCurrent": tsData.WeldingCurrent,
		"Voltage":        tsData.Voltage,
		"WireFeedSpeed":  tsData.WireFeedSpeed,
		"GasFlow":        tsData.GasFlow,
		"TravelSpeed":    tsData.TravelSpeed,
		"PositionX":      tsData.PositionX,
		"PositionY":      tsData.PositionY,
		"PositionZ":      tsData.PositionZ,
		"TorchAngle":     tsData.TorchAngle,
		"State":          int(state.State),
		"GoodParts":      goodParts,
		"ScrapParts":     scrapParts,
		"ArcTime":        arcTime,
		"CycleProgress":  h.stateMachine.GetCycleProgress(),
	}

	if order := h.stateMachine.GetCurrentOrder(); order != nil {
		data["CurrentOrderId"] = order.OrderID
		data["CurrentPartNumber"] = order.PartNumber
	}

	if state.CurrentError != nil {
		data["ErrorCode"] = string(state.CurrentError.Code)
		data["ErrorMessage"] = state.CurrentError.Message
	}

	// Node definitions for welding robot
	nodes := []NodeInfo{
		{Name: "WeldingCurrent", NodeID: "ns=2;s=Robot.WeldingCurrent", DataType: "Double", Unit: "A", Description: "Welding current"},
		{Name: "Voltage", NodeID: "ns=2;s=Robot.Voltage", DataType: "Double", Unit: "V", Description: "Arc voltage"},
		{Name: "WireFeedSpeed", NodeID: "ns=2;s=Robot.WireFeedSpeed", DataType: "Double", Unit: "m/min", Description: "Wire feed speed"},
		{Name: "GasFlow", NodeID: "ns=2;s=Robot.GasFlow", DataType: "Double", Unit: "l/min", Description: "Shielding gas flow"},
		{Name: "TravelSpeed", NodeID: "ns=2;s=Robot.TravelSpeed", DataType: "Double", Unit: "mm/s", Description: "Travel speed"},
		{Name: "Position.X", NodeID: "ns=2;s=Robot.Position.X", DataType: "Double", Unit: "mm", Description: "X position"},
		{Name: "Position.Y", NodeID: "ns=2;s=Robot.Position.Y", DataType: "Double", Unit: "mm", Description: "Y position"},
		{Name: "Position.Z", NodeID: "ns=2;s=Robot.Position.Z", DataType: "Double", Unit: "mm", Description: "Z position"},
		{Name: "TorchAngle", NodeID: "ns=2;s=Robot.TorchAngle", DataType: "Double", Unit: "deg", Description: "Torch angle"},
		{Name: "State", NodeID: "ns=2;s=Robot.State", DataType: "Int32", Description: "Machine state (0-4)"},
		{Name: "GoodParts", NodeID: "ns=2;s=Robot.GoodParts", DataType: "Int32", Description: "Good parts count"},
		{Name: "ScrapParts", NodeID: "ns=2;s=Robot.ScrapParts", DataType: "Int32", Description: "Scrap parts count"},
		{Name: "ArcTime", NodeID: "ns=2;s=Robot.ArcTime", DataType: "Double", Unit: "s", Description: "Cumulative arc time"},
		{Name: "CycleProgress", NodeID: "ns=2;s=Robot.CycleProgress", DataType: "Double", Unit: "%", Description: "Cycle progress"},
		{Name: "CurrentOrderId", NodeID: "ns=2;s=Robot.CurrentOrderId", DataType: "String", Description: "Active order ID"},
		{Name: "ErrorCode", NodeID: "ns=2;s=Robot.ErrorCode", DataType: "String", Description: "Current error code"},
		{Name: "ErrorMessage", NodeID: "ns=2;s=Robot.ErrorMessage", DataType: "String", Description: "Error description"},
	}

	return &MachineDetailResponse{
		ID:        "welding",
		Name:      "WeldingRobot",
		Type:      "Welding Robot",
		Namespace: core.NamespaceWelding,
		State:     int(state.State),
		StateName: state.State.String(),
		Data:      data,
		Nodes:     nodes,
	}
}

func (h *Handler) writeJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	if err := json.NewEncoder(w).Encode(data); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// HandleConfig handles GET and POST /api/config
func (h *Handler) HandleConfig(w http.ResponseWriter, r *http.Request) {
	// Handle CORS preflight
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.WriteHeader(http.StatusOK)
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.handleConfigGet(w, r)
	case http.MethodPost:
		h.handleConfigUpdate(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handler) handleConfigGet(w http.ResponseWriter, r *http.Request) {
	if h.runner == nil {
		http.Error(w, "Config only available in production line mode", http.StatusNotImplemented)
		return
	}

	rc := h.runner.GetRuntimeConfig()
	snapshot := rc.Snapshot()

	resp := ConfigResponse{
		CycleTimeScale:     snapshot.CycleTimeScale,
		BaseCycleTime:      snapshot.BaseCycleTime.String(),
		EffectiveCycleTime: snapshot.EffectiveCycleTime.String(),
		ScrapRate:          snapshot.ScrapRate,
		ErrorRate:          snapshot.ErrorRate,
	}

	h.writeJSON(w, resp)
}

func (h *Handler) handleConfigUpdate(w http.ResponseWriter, r *http.Request) {
	if h.runner == nil {
		http.Error(w, "Config only available in production line mode", http.StatusNotImplemented)
		return
	}

	var req ConfigUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	rc := h.runner.GetRuntimeConfig()

	// Apply updates
	if req.CycleTimeScale != nil {
		if err := rc.SetCycleTimeScale(*req.CycleTimeScale); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}

	if req.ScrapRate != nil {
		if err := rc.SetScrapRate(*req.ScrapRate); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}

	if req.ErrorRate != nil {
		if err := rc.SetErrorRate(*req.ErrorRate); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}

	// Return updated config
	snapshot := rc.Snapshot()
	resp := ConfigResponse{
		CycleTimeScale:     snapshot.CycleTimeScale,
		BaseCycleTime:      snapshot.BaseCycleTime.String(),
		EffectiveCycleTime: snapshot.EffectiveCycleTime.String(),
		ScrapRate:          snapshot.ScrapRate,
		ErrorRate:          snapshot.ErrorRate,
	}

	h.writeJSON(w, resp)
}
