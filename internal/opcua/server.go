package opcua

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/awcullen/opcua/server"
	"github.com/awcullen/opcua/ua"
	"github.com/rs/zerolog/log"

	"github.com/sebastiankruger/shopfloor-simulator/internal/core"
	"github.com/sebastiankruger/shopfloor-simulator/internal/simulator"
)

const (
	pkiDir   = "./pki"
	certFile = "./pki/server.crt"
	keyFile  = "./pki/server.key"
)

// NamespaceNodes holds nodes for a specific namespace
type NamespaceNodes struct {
	Namespace   uint16
	FolderName  string
	FolderDesc  string
	NodeDefs    []core.NodeDefinition // Store definitions for deferred registration
	VarNodes    map[string]*server.VariableNode
	Values      map[string]interface{}
}

// Server wraps the OPC UA server and manages node values for multiple namespaces
type Server struct {
	srv  *server.Server
	port int
	mu   sync.RWMutex

	// Multi-namespace support
	namespaces map[uint16]*NamespaceNodes

	// Legacy single-namespace support (for backward compatibility)
	namespace uint16
	nodes     map[string]*NodeInfo
	varNodes  map[string]*server.VariableNode

	// Legacy node references for welding robot
	currentNode       ua.NodeID
	voltageNode       ua.NodeID
	wireFeedNode      ua.NodeID
	gasFlowNode       ua.NodeID
	travelSpeedNode   ua.NodeID
	arcTimeNode       ua.NodeID
	posXNode          ua.NodeID
	posYNode          ua.NodeID
	posZNode          ua.NodeID
	torchAngleNode    ua.NodeID
	stateNode         ua.NodeID
	goodPartsNode     ua.NodeID
	scrapPartsNode    ua.NodeID
	orderIdNode       ua.NodeID
	partNumberNode    ua.NodeID
	cycleProgressNode ua.NodeID
	errorCodeNode     ua.NodeID
	errorMessageNode  ua.NodeID
	errorTimeNode     ua.NodeID
}

// NodeInfo holds information about an OPC UA node
type NodeInfo struct {
	NodeID   ua.NodeID
	Name     string
	DataType ua.NodeID
	Value    interface{}
}

// NewServer creates a new OPC UA server
func NewServer(port int, simulatorName string) (*Server, error) {
	s := &Server{
		port:       port,
		namespaces: make(map[uint16]*NamespaceNodes),
		nodes:      make(map[string]*NodeInfo),
		varNodes:   make(map[string]*server.VariableNode),
	}

	return s, nil
}

// ensurePKI creates PKI directory and self-signed certificates if they don't exist
func ensurePKI(appName string) error {
	// Check if cert already exists
	if _, err := os.Stat(certFile); err == nil {
		log.Info().Str("certFile", certFile).Msg("Using existing PKI certificates")
		return nil
	}

	log.Info().Msg("Generating self-signed certificates for OPC UA server")

	// Create PKI directory
	if err := os.MkdirAll(pkiDir, 0755); err != nil {
		return fmt.Errorf("failed to create PKI directory: %w", err)
	}

	// Generate self-signed certificate
	return createSelfSignedCert(appName, certFile, keyFile)
}

// createSelfSignedCert generates a self-signed certificate for OPC UA server
func createSelfSignedCert(appName, certPath, keyPath string) error {
	// Generate RSA key pair
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return fmt.Errorf("failed to generate private key: %w", err)
	}

	// Create certificate template
	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return fmt.Errorf("failed to generate serial number: %w", err)
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName:   appName,
			Organization: []string{"Shopfloor Simulator"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour), // 1 year validity
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
		DNSNames:              []string{"localhost", appName, "production-line-simulator", "shopfloor-simulator"},
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("0.0.0.0")},
	}

	// Add OPC UA application URI as SAN
	template.URIs = []*url.URL{
		{Scheme: "urn", Opaque: "shopfloor-simulator:production-line"},
	}

	// Create self-signed certificate
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return fmt.Errorf("failed to create certificate: %w", err)
	}

	// Write certificate to file
	certFileHandle, err := os.Create(certPath)
	if err != nil {
		return fmt.Errorf("failed to create cert file: %w", err)
	}
	defer certFileHandle.Close()

	if err := pem.Encode(certFileHandle, &pem.Block{Type: "CERTIFICATE", Bytes: certDER}); err != nil {
		return fmt.Errorf("failed to encode certificate: %w", err)
	}

	// Write private key to file
	keyFileHandle, err := os.Create(keyPath)
	if err != nil {
		return fmt.Errorf("failed to create key file: %w", err)
	}
	defer keyFileHandle.Close()

	keyDER := x509.MarshalPKCS1PrivateKey(privateKey)
	if err := pem.Encode(keyFileHandle, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: keyDER}); err != nil {
		return fmt.Errorf("failed to encode private key: %w", err)
	}

	log.Info().
		Str("certPath", certPath).
		Str("keyPath", keyPath).
		Msg("Self-signed certificates generated successfully")

	return nil
}

// Start starts the OPC UA server
func (s *Server) Start(ctx context.Context) error {
	endpoint := fmt.Sprintf("opc.tcp://0.0.0.0:%d", s.port)

	log.Info().
		Int("port", s.port).
		Str("endpoint", endpoint).
		Msg("Starting OPC UA server")

	// Initialize legacy node references (for backward compatibility)
	s.initializeNodeReferences()

	// Generate self-signed certificates if needed
	if err := ensurePKI("ProductionLineSimulator"); err != nil {
		log.Warn().Err(err).Msg("Failed to create PKI - OPC UA server disabled")
		log.Info().Msg("OPC UA server disabled - running simulator in data generation mode only")
		return nil
	}

	// Try to create the OPC UA server with panic recovery
	var srv *server.Server
	func() {
		defer func() {
			if r := recover(); r != nil {
				log.Warn().
					Interface("panic", r).
					Msg("OPC UA server creation panicked - running in value storage mode only")
			}
		}()

		var err error
		srv, err = server.New(
			ua.ApplicationDescription{
				ApplicationURI:  "urn:shopfloor-simulator:production-line",
				ProductURI:      "urn:shopfloor-simulator",
				ApplicationName: ua.LocalizedText{Text: "Production Line Simulator", Locale: "en"},
				ApplicationType: ua.ApplicationTypeServer,
			},
			certFile, // Self-signed certificate
			keyFile,  // Private key
			endpoint,
			server.WithAnonymousIdentity(true),
			server.WithSecurityPolicyNone(true),
			server.WithInsecureSkipVerify(),
		)
		if err != nil {
			log.Warn().
				Err(err).
				Msg("OPC UA server creation failed - running in value storage mode only")
			srv = nil
		}
	}()

	if srv == nil {
		log.Info().Msg("OPC UA server disabled - running simulator in data generation mode only")
		return nil
	}

	s.srv = srv

	// Check if there are pending namespaces to register (from production line mode)
	if len(s.namespaces) > 0 {
		// Re-register all pending namespaces now that the server is available
		if err := s.registerPendingNamespaces(); err != nil {
			log.Error().Err(err).Msg("Failed to register pending namespaces")
			return err
		}
	} else {
		// Register legacy welding nodes for backward compatibility
		if err := s.createNodes(); err != nil {
			log.Error().Err(err).Msg("Failed to create OPC UA nodes")
			return err
		}
	}

	// Start server in background
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Error().Interface("panic", r).Msg("OPC UA server panic")
			}
		}()
		if err := srv.ListenAndServe(); err != nil {
			log.Error().Err(err).Msg("OPC UA server error")
		}
	}()

	log.Info().Msg("OPC UA server started successfully")
	return nil
}

// Stop stops the OPC UA server
func (s *Server) Stop(ctx context.Context) error {
	if s.srv != nil {
		return s.srv.Close()
	}
	return nil
}

// RegisterNamespace creates a new namespace with a root folder and variable nodes
func (s *Server) RegisterNamespace(nsIndex uint16, folderName, folderDesc string, nodes []core.NodeDefinition) error {
	if s.srv == nil {
		// Store namespace info for deferred registration when server starts
		ns := &NamespaceNodes{
			Namespace:  nsIndex,
			FolderName: folderName,
			FolderDesc: folderDesc,
			NodeDefs:   nodes, // Store for later registration
			VarNodes:   make(map[string]*server.VariableNode),
			Values:     make(map[string]interface{}),
		}
		for _, nodeDef := range nodes {
			ns.Values[nodeDef.Name] = nodeDef.InitialValue
		}
		s.namespaces[nsIndex] = ns
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	nm := s.srv.NamespaceManager()

	// Create folder node under Objects folder
	folder := server.NewObjectNode(
		s.srv,
		ua.NodeIDString{NamespaceIndex: nsIndex, ID: folderName},
		ua.QualifiedName{NamespaceIndex: nsIndex, Name: folderName},
		ua.LocalizedText{Text: folderName},
		ua.LocalizedText{Text: folderDesc},
		nil,
		[]ua.Reference{
			{
				ReferenceTypeID: ua.ReferenceTypeIDOrganizes,
				IsInverse:       true,
				TargetID:        ua.ExpandedNodeID{NodeID: ua.ObjectIDObjectsFolder},
			},
		},
		0,
	)
	nm.AddNode(folder)

	// Create namespace nodes storage
	ns := &NamespaceNodes{
		Namespace:  nsIndex,
		FolderName: folderName,
		VarNodes:   make(map[string]*server.VariableNode),
		Values:     make(map[string]interface{}),
	}

	// Create variable nodes
	for _, nodeDef := range nodes {
		varNode := server.NewVariableNode(
			s.srv,
			ua.NodeIDString{NamespaceIndex: nsIndex, ID: folderName + "." + nodeDef.Name},
			ua.QualifiedName{NamespaceIndex: nsIndex, Name: nodeDef.Name},
			ua.LocalizedText{Text: nodeDef.DisplayName},
			ua.LocalizedText{Text: nodeDef.Description},
			nil,
			[]ua.Reference{
				{
					ReferenceTypeID: ua.ReferenceTypeIDHasComponent,
					IsInverse:       true,
					TargetID:        ua.ExpandedNodeID{NodeID: ua.NodeIDString{NamespaceIndex: nsIndex, ID: folderName}},
				},
			},
			ua.NewDataValue(nodeDef.InitialValue, 0, time.Now().UTC(), 0, time.Now().UTC(), 0),
			core.OPCUADataType(nodeDef.DataType),
			ua.ValueRankScalar,
			[]uint32{},
			ua.AccessLevelsCurrentRead,
			250.0,
			false,
			nil,
		)
		nm.AddNode(varNode)
		ns.VarNodes[nodeDef.Name] = varNode
		ns.Values[nodeDef.Name] = nodeDef.InitialValue
	}

	s.namespaces[nsIndex] = ns

	log.Info().
		Uint16("namespace", nsIndex).
		Str("folder", folderName).
		Int("nodes", len(nodes)).
		Msg("Registered OPC UA namespace")

	return nil
}

// UpdateNamespaceValues updates all values for a namespace
func (s *Server) UpdateNamespaceValues(nsIndex uint16, values map[string]interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()

	ns, ok := s.namespaces[nsIndex]
	if !ok {
		return
	}

	now := time.Now().UTC()
	for name, value := range values {
		ns.Values[name] = value
		if varNode, ok := ns.VarNodes[name]; ok {
			varNode.SetValue(ua.NewDataValue(value, 0, now, 0, now, 0))
		}
	}
}

// registerPendingNamespaces registers all stored namespaces after server is available
func (s *Server) registerPendingNamespaces() error {
	nm := s.srv.NamespaceManager()
	nodeCount := 0

	for nsIndex, ns := range s.namespaces {
		// Create folder node under Objects folder
		folder := server.NewObjectNode(
			s.srv,
			ua.NodeIDString{NamespaceIndex: nsIndex, ID: ns.FolderName},
			ua.QualifiedName{NamespaceIndex: nsIndex, Name: ns.FolderName},
			ua.LocalizedText{Text: ns.FolderName},
			ua.LocalizedText{Text: ns.FolderDesc},
			nil,
			[]ua.Reference{
				{
					ReferenceTypeID: ua.ReferenceTypeIDOrganizes,
					IsInverse:       true,
					TargetID:        ua.ExpandedNodeID{NodeID: ua.ObjectIDObjectsFolder},
				},
			},
			0,
		)
		nm.AddNode(folder)

		// Create variable nodes from stored definitions
		for _, nodeDef := range ns.NodeDefs {
			varNode := server.NewVariableNode(
				s.srv,
				ua.NodeIDString{NamespaceIndex: nsIndex, ID: ns.FolderName + "." + nodeDef.Name},
				ua.QualifiedName{NamespaceIndex: nsIndex, Name: nodeDef.Name},
				ua.LocalizedText{Text: nodeDef.DisplayName},
				ua.LocalizedText{Text: nodeDef.Description},
				nil,
				[]ua.Reference{
					{
						ReferenceTypeID: ua.ReferenceTypeIDHasComponent,
						IsInverse:       true,
						TargetID:        ua.ExpandedNodeID{NodeID: ua.NodeIDString{NamespaceIndex: nsIndex, ID: ns.FolderName}},
					},
				},
				ua.NewDataValue(nodeDef.InitialValue, 0, time.Now().UTC(), 0, time.Now().UTC(), 0),
				core.OPCUADataType(nodeDef.DataType),
				ua.ValueRankScalar,
				[]uint32{},
				ua.AccessLevelsCurrentRead,
				250.0,
				false,
				nil,
			)
			nm.AddNode(varNode)
			ns.VarNodes[nodeDef.Name] = varNode
			nodeCount++
		}

		log.Info().
			Uint16("namespace", nsIndex).
			Str("folder", ns.FolderName).
			Int("nodes", len(ns.NodeDefs)).
			Msg("Registered OPC UA namespace")
	}

	log.Info().Int("count", nodeCount).Msg("OPC UA nodes registered in address space")
	return nil
}

// GetNamespaceValue returns a value from a namespace
func (s *Server) GetNamespaceValue(nsIndex uint16, name string) (interface{}, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ns, ok := s.namespaces[nsIndex]
	if !ok {
		return nil, false
	}

	value, ok := ns.Values[name]
	return value, ok
}

// Legacy methods for backward compatibility with welding robot

func (s *Server) createNodes() error {
	s.namespace = 2 // Use namespace index 2 for our custom nodes
	nm := s.srv.NamespaceManager()

	// Create folder node for Robot under Objects folder
	robotFolder := server.NewObjectNode(
		s.srv,
		ua.NodeIDString{NamespaceIndex: 2, ID: "Robot"},
		ua.QualifiedName{NamespaceIndex: 2, Name: "Robot"},
		ua.LocalizedText{Text: "Robot"},
		ua.LocalizedText{Text: "Welding Robot Data"},
		nil,
		[]ua.Reference{
			{
				ReferenceTypeID: ua.ReferenceTypeIDOrganizes,
				IsInverse:       true,
				TargetID:        ua.ExpandedNodeID{NodeID: ua.ObjectIDObjectsFolder},
			},
		},
		0,
	)
	nm.AddNode(robotFolder)

	// Helper function to create a variable node
	createVar := func(name, displayName, description string, dataType ua.NodeID, initialValue interface{}) *server.VariableNode {
		return server.NewVariableNode(
			s.srv,
			ua.NodeIDString{NamespaceIndex: 2, ID: "Robot." + name},
			ua.QualifiedName{NamespaceIndex: 2, Name: name},
			ua.LocalizedText{Text: displayName},
			ua.LocalizedText{Text: description},
			nil,
			[]ua.Reference{
				{
					ReferenceTypeID: ua.ReferenceTypeIDHasComponent,
					IsInverse:       true,
					TargetID:        ua.ExpandedNodeID{NodeID: ua.NodeIDString{NamespaceIndex: 2, ID: "Robot"}},
				},
			},
			ua.NewDataValue(initialValue, 0, time.Now().UTC(), 0, time.Now().UTC(), 0),
			dataType,
			ua.ValueRankScalar,
			[]uint32{},
			ua.AccessLevelsCurrentRead,
			250.0,
			false,
			nil,
		)
	}

	// Create all welding parameter nodes
	nodes := []*server.VariableNode{
		createVar("WeldingCurrent", "Welding Current", "Current in Amps", ua.DataTypeIDDouble, 0.0),
		createVar("Voltage", "Voltage", "Arc voltage in Volts", ua.DataTypeIDDouble, 0.0),
		createVar("WireFeedSpeed", "Wire Feed Speed", "Wire feed in m/min", ua.DataTypeIDDouble, 0.0),
		createVar("GasFlow", "Gas Flow", "Shielding gas flow l/min", ua.DataTypeIDDouble, 0.0),
		createVar("TravelSpeed", "Travel Speed", "Travel speed mm/s", ua.DataTypeIDDouble, 0.0),
		createVar("ArcTime", "Arc Time", "Cumulative arc time seconds", ua.DataTypeIDDouble, 0.0),
		createVar("Position.X", "Position X", "X position mm", ua.DataTypeIDDouble, 0.0),
		createVar("Position.Y", "Position Y", "Y position mm", ua.DataTypeIDDouble, 0.0),
		createVar("Position.Z", "Position Z", "Z position mm", ua.DataTypeIDDouble, 200.0),
		createVar("TorchAngle", "Torch Angle", "Torch angle degrees", ua.DataTypeIDDouble, 0.0),
		createVar("State", "State", "Machine state (0-4)", ua.DataTypeIDInt32, int32(0)),
		createVar("GoodParts", "Good Parts", "Good parts count", ua.DataTypeIDInt32, int32(0)),
		createVar("ScrapParts", "Scrap Parts", "Scrap parts count", ua.DataTypeIDInt32, int32(0)),
		createVar("CurrentOrderId", "Current Order ID", "Active order ID", ua.DataTypeIDString, ""),
		createVar("CurrentPartNumber", "Current Part Number", "Active part number", ua.DataTypeIDString, ""),
		createVar("CycleProgress", "Cycle Progress", "Progress 0-100%", ua.DataTypeIDDouble, 0.0),
		createVar("ErrorCode", "Error Code", "Current error code", ua.DataTypeIDString, ""),
		createVar("ErrorMessage", "Error Message", "Error description", ua.DataTypeIDString, ""),
	}

	// Register nodes and store references
	for _, node := range nodes {
		nm.AddNode(node)
		// Extract name from NodeID for lookup
		nodeID := node.NodeID().(ua.NodeIDString)
		name := strings.TrimPrefix(nodeID.ID, "Robot.")
		s.varNodes[name] = node
	}

	log.Info().Int("count", len(nodes)).Msg("OPC UA nodes registered in address space")
	return nil
}

func (s *Server) initializeNodeReferences() {
	// Create string node IDs matching our spec
	ns := s.namespace

	s.currentNode = ua.NewNodeIDString(ns, "Robot.WeldingCurrent")
	s.voltageNode = ua.NewNodeIDString(ns, "Robot.Voltage")
	s.wireFeedNode = ua.NewNodeIDString(ns, "Robot.WireFeedSpeed")
	s.gasFlowNode = ua.NewNodeIDString(ns, "Robot.GasFlow")
	s.travelSpeedNode = ua.NewNodeIDString(ns, "Robot.TravelSpeed")
	s.arcTimeNode = ua.NewNodeIDString(ns, "Robot.ArcTime")
	s.posXNode = ua.NewNodeIDString(ns, "Robot.Position.X")
	s.posYNode = ua.NewNodeIDString(ns, "Robot.Position.Y")
	s.posZNode = ua.NewNodeIDString(ns, "Robot.Position.Z")
	s.torchAngleNode = ua.NewNodeIDString(ns, "Robot.TorchAngle")
	s.stateNode = ua.NewNodeIDString(ns, "Robot.State")
	s.goodPartsNode = ua.NewNodeIDString(ns, "Robot.GoodParts")
	s.scrapPartsNode = ua.NewNodeIDString(ns, "Robot.ScrapParts")
	s.orderIdNode = ua.NewNodeIDString(ns, "Robot.CurrentOrderId")
	s.partNumberNode = ua.NewNodeIDString(ns, "Robot.CurrentPartNumber")
	s.cycleProgressNode = ua.NewNodeIDString(ns, "Robot.CycleProgress")
	s.errorCodeNode = ua.NewNodeIDString(ns, "Robot.ErrorCode")
	s.errorMessageNode = ua.NewNodeIDString(ns, "Robot.ErrorMessage")
	s.errorTimeNode = ua.NewNodeIDString(ns, "Robot.ErrorTimestamp")

	// Initialize node info map
	s.nodes["WeldingCurrent"] = &NodeInfo{NodeID: s.currentNode, Name: "WeldingCurrent", Value: 0.0}
	s.nodes["Voltage"] = &NodeInfo{NodeID: s.voltageNode, Name: "Voltage", Value: 0.0}
	s.nodes["WireFeedSpeed"] = &NodeInfo{NodeID: s.wireFeedNode, Name: "WireFeedSpeed", Value: 0.0}
	s.nodes["GasFlow"] = &NodeInfo{NodeID: s.gasFlowNode, Name: "GasFlow", Value: 0.0}
	s.nodes["TravelSpeed"] = &NodeInfo{NodeID: s.travelSpeedNode, Name: "TravelSpeed", Value: 0.0}
	s.nodes["ArcTime"] = &NodeInfo{NodeID: s.arcTimeNode, Name: "ArcTime", Value: 0.0}
	s.nodes["PositionX"] = &NodeInfo{NodeID: s.posXNode, Name: "PositionX", Value: 0.0}
	s.nodes["PositionY"] = &NodeInfo{NodeID: s.posYNode, Name: "PositionY", Value: 0.0}
	s.nodes["PositionZ"] = &NodeInfo{NodeID: s.posZNode, Name: "PositionZ", Value: 200.0}
	s.nodes["TorchAngle"] = &NodeInfo{NodeID: s.torchAngleNode, Name: "TorchAngle", Value: 0.0}
	s.nodes["State"] = &NodeInfo{NodeID: s.stateNode, Name: "State", Value: int32(0)}
	s.nodes["GoodParts"] = &NodeInfo{NodeID: s.goodPartsNode, Name: "GoodParts", Value: int32(0)}
	s.nodes["ScrapParts"] = &NodeInfo{NodeID: s.scrapPartsNode, Name: "ScrapParts", Value: int32(0)}
	s.nodes["CurrentOrderId"] = &NodeInfo{NodeID: s.orderIdNode, Name: "CurrentOrderId", Value: ""}
	s.nodes["CurrentPartNumber"] = &NodeInfo{NodeID: s.partNumberNode, Name: "CurrentPartNumber", Value: ""}
	s.nodes["CycleProgress"] = &NodeInfo{NodeID: s.cycleProgressNode, Name: "CycleProgress", Value: 0.0}
	s.nodes["ErrorCode"] = &NodeInfo{NodeID: s.errorCodeNode, Name: "ErrorCode", Value: ""}
	s.nodes["ErrorMessage"] = &NodeInfo{NodeID: s.errorMessageNode, Name: "ErrorMessage", Value: ""}
	s.nodes["ErrorTimestamp"] = &NodeInfo{NodeID: s.errorTimeNode, Name: "ErrorTimestamp", Value: time.Time{}}
}

// setNodeValue sets the value of an OPC UA variable node
func (s *Server) setNodeValue(name string, value interface{}, timestamp time.Time) {
	if node, ok := s.varNodes[name]; ok {
		node.SetValue(ua.NewDataValue(value, 0, timestamp, 0, timestamp, 0))
	}
}

// UpdateValues updates all OPC UA node values from timeseries data (legacy method)
func (s *Server) UpdateValues(data *simulator.TimeseriesData) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Update stored values (keep for fallback/local access)
	s.nodes["WeldingCurrent"].Value = data.WeldingCurrent
	s.nodes["Voltage"].Value = data.Voltage
	s.nodes["WireFeedSpeed"].Value = data.WireFeedSpeed
	s.nodes["GasFlow"].Value = data.GasFlow
	s.nodes["TravelSpeed"].Value = data.TravelSpeed
	s.nodes["ArcTime"].Value = data.ArcTime
	s.nodes["PositionX"].Value = data.PositionX
	s.nodes["PositionY"].Value = data.PositionY
	s.nodes["PositionZ"].Value = data.PositionZ
	s.nodes["TorchAngle"].Value = data.TorchAngle
	s.nodes["State"].Value = int32(data.State)
	s.nodes["GoodParts"].Value = int32(data.GoodParts)
	s.nodes["ScrapParts"].Value = int32(data.ScrapParts)
	s.nodes["CurrentOrderId"].Value = data.CurrentOrderID
	s.nodes["CurrentPartNumber"].Value = data.CurrentPartNumber
	s.nodes["CycleProgress"].Value = data.CycleProgress
	s.nodes["ErrorCode"].Value = data.ErrorCode
	s.nodes["ErrorMessage"].Value = data.ErrorMessage
	s.nodes["ErrorTimestamp"].Value = data.ErrorTimestamp

	// Update OPC UA server nodes (if server is running)
	if s.srv != nil && len(s.varNodes) > 0 {
		now := time.Now().UTC()

		s.setNodeValue("WeldingCurrent", data.WeldingCurrent, now)
		s.setNodeValue("Voltage", data.Voltage, now)
		s.setNodeValue("WireFeedSpeed", data.WireFeedSpeed, now)
		s.setNodeValue("GasFlow", data.GasFlow, now)
		s.setNodeValue("TravelSpeed", data.TravelSpeed, now)
		s.setNodeValue("ArcTime", data.ArcTime, now)
		s.setNodeValue("Position.X", data.PositionX, now)
		s.setNodeValue("Position.Y", data.PositionY, now)
		s.setNodeValue("Position.Z", data.PositionZ, now)
		s.setNodeValue("TorchAngle", data.TorchAngle, now)
		s.setNodeValue("State", int32(data.State), now)
		s.setNodeValue("GoodParts", int32(data.GoodParts), now)
		s.setNodeValue("ScrapParts", int32(data.ScrapParts), now)
		s.setNodeValue("CurrentOrderId", data.CurrentOrderID, now)
		s.setNodeValue("CurrentPartNumber", data.CurrentPartNumber, now)
		s.setNodeValue("CycleProgress", data.CycleProgress, now)
		s.setNodeValue("ErrorCode", data.ErrorCode, now)
		s.setNodeValue("ErrorMessage", data.ErrorMessage, now)
	}
}

// GetNodeValue returns the current value of a node (legacy method)
func (s *Server) GetNodeValue(name string) (interface{}, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if node, ok := s.nodes[name]; ok {
		return node.Value, true
	}
	return nil, false
}

// GetAllValues returns all current node values as a map (legacy method)
func (s *Server) GetAllValues() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	values := make(map[string]interface{})
	for name, node := range s.nodes {
		values[name] = node.Value
	}
	return values
}
