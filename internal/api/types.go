package api

// StatusResponse is returned by GET /api/status
type StatusResponse struct {
	Mode          string           `json:"mode"`
	SimulatorName string           `json:"simulatorName"`
	Machines      []MachineInfo    `json:"machines"`
	CurrentOrder  *OrderInfo       `json:"currentOrder,omitempty"`
}

// MachineInfo provides basic info about a machine
type MachineInfo struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Type      string `json:"type"`
	Namespace uint16 `json:"namespace"`
}

// OrderInfo provides current order information
type OrderInfo struct {
	OrderID    string `json:"orderId"`
	PartNumber string `json:"partNumber"`
	Quantity   int    `json:"quantity"`
	Completed  int    `json:"completed"`
	Scrap      int    `json:"scrap"`
	Status     string `json:"status"`
}

// MachineDetailResponse is returned by GET /api/machines/{id}
type MachineDetailResponse struct {
	ID        string                 `json:"id"`
	Name      string                 `json:"name"`
	Type      string                 `json:"type"`
	Namespace uint16                 `json:"namespace"`
	State     int                    `json:"state"`
	StateName string                 `json:"stateName"`
	Data      map[string]interface{} `json:"data"`
	Nodes     []NodeInfo             `json:"nodes"`
}

// MachineListResponse is returned by GET /api/machines
type MachineListResponse struct {
	Machines []MachineSummary `json:"machines"`
}

// MachineSummary provides a summary of a machine's current state
type MachineSummary struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	Type          string  `json:"type"`
	Namespace     uint16  `json:"namespace"`
	State         int     `json:"state"`
	StateName     string  `json:"stateName"`
	GoodParts     int     `json:"goodParts"`
	ScrapParts    int     `json:"scrapParts"`
	CycleProgress float64 `json:"cycleProgress"`
}

// NodeInfo describes an OPC UA node
type NodeInfo struct {
	Name        string `json:"name"`
	NodeID      string `json:"nodeId"`
	DataType    string `json:"dataType"`
	Unit        string `json:"unit,omitempty"`
	Description string `json:"description,omitempty"`
}

// DataTypeToString converts internal data type to string representation
func DataTypeToString(dt int) string {
	switch dt {
	case 0:
		return "Double"
	case 1:
		return "Float"
	case 2:
		return "Int32"
	case 3:
		return "Int64"
	case 4:
		return "String"
	case 5:
		return "Boolean"
	case 6:
		return "DateTime"
	default:
		return "Unknown"
	}
}

// ConfigResponse is returned by GET /api/config
type ConfigResponse struct {
	CycleTimeScale     float64 `json:"cycleTimeScale"`
	BaseCycleTime      string  `json:"baseCycleTime"`
	EffectiveCycleTime string  `json:"effectiveCycleTime"`
	ScrapRate          float64 `json:"scrapRate"`
	ErrorRate          float64 `json:"errorRate"`
}

// ConfigUpdateRequest is used for POST /api/config
type ConfigUpdateRequest struct {
	CycleTimeScale *float64 `json:"cycleTimeScale,omitempty"`
	ScrapRate      *float64 `json:"scrapRate,omitempty"`
	ErrorRate      *float64 `json:"errorRate,omitempty"`
}
