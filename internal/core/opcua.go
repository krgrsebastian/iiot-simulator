package core

import (
	"github.com/awcullen/opcua/ua"
)

// NamespaceInfo describes a namespace in the OPC UA server
type NamespaceInfo struct {
	Index      uint16 // Namespace index (2, 3, 4, 5, etc.)
	URI        string // Namespace URI
	FolderName string // Root folder name (e.g., "FormingMachine", "PickerRobot")
	FolderDesc string // Folder description
}

// Predefined namespace indices for the production line
const (
	NamespaceForming    uint16 = 2 // Forming Machine
	NamespacePicker     uint16 = 3 // Picker Robot
	NamespaceSpotWelder uint16 = 4 // Spot Welder
	NamespaceLine       uint16 = 5 // Production Line (coordinator)

	// For standalone welding robot (legacy compatibility)
	NamespaceWelding uint16 = 2
)

// OPCUADataType converts core.DataType to ua.NodeID
func OPCUADataType(dt DataType) ua.NodeID {
	switch dt {
	case DataTypeDouble:
		return ua.DataTypeIDDouble
	case DataTypeFloat:
		return ua.DataTypeIDFloat
	case DataTypeInt32:
		return ua.DataTypeIDInt32
	case DataTypeInt64:
		return ua.DataTypeIDInt64
	case DataTypeString:
		return ua.DataTypeIDString
	case DataTypeBool:
		return ua.DataTypeIDBoolean
	case DataTypeDateTime:
		return ua.DataTypeIDDateTime
	default:
		return ua.DataTypeIDString
	}
}
