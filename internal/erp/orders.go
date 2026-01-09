package erp

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/sebastiankruger/shopfloor-simulator/internal/config"
	"github.com/sebastiankruger/shopfloor-simulator/internal/simulator"
)

// PartCatalog defines available parts for production
var PartCatalog = []simulator.PartDefinition{
	{PartNumber: "WLD-FRAME-A01", Description: "Front Frame Assembly", CycleTime: 55 * time.Second},
	{PartNumber: "WLD-FRAME-B02", Description: "Rear Frame Assembly", CycleTime: 70 * time.Second},
	{PartNumber: "WLD-BRACKET-C01", Description: "Support Bracket", CycleTime: 35 * time.Second},
	{PartNumber: "WLD-PANEL-D01", Description: "Side Panel", CycleTime: 45 * time.Second},
	{PartNumber: "WLD-MOUNT-E01", Description: "Motor Mount", CycleTime: 40 * time.Second},
	{PartNumber: "WLD-CROSS-F01", Description: "Cross Member", CycleTime: 60 * time.Second},
}

// CustomerList defines customers for order generation
var CustomerList = []string{
	"AutoCorp Inc.",
	"MechParts GmbH",
	"TechFab Solutions",
	"Industrial Motors Ltd.",
	"Assembly Systems AG",
}

// OrderGenerator generates production orders automatically
type OrderGenerator struct {
	cfg         *config.Config
	rng         *rand.Rand
	orderNumber int
}

// NewOrderGenerator creates a new order generator
func NewOrderGenerator(cfg *config.Config) *OrderGenerator {
	return &OrderGenerator{
		cfg:         cfg,
		rng:         rand.New(rand.NewSource(time.Now().UnixNano())),
		orderNumber: 1000,
	}
}

// GenerateOrder creates a new random production order
func (og *OrderGenerator) GenerateOrder() *simulator.ProductionOrder {
	// Select random part
	part := PartCatalog[og.rng.Intn(len(PartCatalog))]

	// Select random customer
	customer := CustomerList[og.rng.Intn(len(CustomerList))]

	// Generate random quantity within configured range
	quantity := og.cfg.OrderMinQty + og.rng.Intn(og.cfg.OrderMaxQty-og.cfg.OrderMinQty+1)

	// Calculate due date (8-48 hours from now)
	hoursUntilDue := 8 + og.rng.Intn(40)
	dueDate := time.Now().Add(time.Duration(hoursUntilDue) * time.Hour)

	// Generate priority (1=Urgent, 2=High, 3=Normal, 4=Low)
	priority := 1 + og.rng.Intn(4)

	og.orderNumber++
	orderID := fmt.Sprintf("PO-%d-%05d", time.Now().Year(), og.orderNumber)

	return &simulator.ProductionOrder{
		OrderID:           orderID,
		PartNumber:        part.PartNumber,
		PartDescription:   part.Description,
		Quantity:          quantity,
		QuantityCompleted: 0,
		QuantityScrap:     0,
		DueDate:           dueDate,
		Customer:          customer,
		Priority:          priority,
		Status:            simulator.OrderStatusQueued,
	}
}

// GenerateInitialQueue generates an initial queue of orders
func (og *OrderGenerator) GenerateInitialQueue(count int) []*simulator.ProductionOrder {
	orders := make([]*simulator.ProductionOrder, count)
	for i := 0; i < count; i++ {
		orders[i] = og.GenerateOrder()
	}
	return orders
}

// GetPartCycleTime returns the cycle time for a specific part number
func GetPartCycleTime(partNumber string, defaultCycleTime time.Duration) time.Duration {
	for _, part := range PartCatalog {
		if part.PartNumber == partNumber {
			return part.CycleTime
		}
	}
	return defaultCycleTime
}
