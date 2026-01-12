package line

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/sebastiankruger/shopfloor-simulator/internal/config"
	"github.com/sebastiankruger/shopfloor-simulator/internal/core"
)

// LineParts defines parts for the production line (forming → spot welding)
var LineParts = []core.PartDefinition{
	{PartNumber: "RAIL-ASM-A01", Description: "Side Rail Assembly", CycleTime: 45 * time.Second},
	{PartNumber: "RAIL-ASM-B02", Description: "Cross Rail Assembly", CycleTime: 50 * time.Second},
	{PartNumber: "BRKT-FRM-C01", Description: "Formed Bracket", CycleTime: 35 * time.Second},
	{PartNumber: "PANEL-WLD-D01", Description: "Welded Panel", CycleTime: 55 * time.Second},
	{PartNumber: "MOUNT-ASM-E01", Description: "Mount Assembly", CycleTime: 40 * time.Second},
}

// LineCustomers defines customers for production line orders
var LineCustomers = []string{
	"AutoCorp Inc.",
	"MechParts GmbH",
	"TechFab Solutions",
	"Industrial Motors Ltd.",
	"Assembly Systems AG",
}

// OrderGenerator generates production orders for the line
type OrderGenerator struct {
	cfg         *config.Config
	rng         *rand.Rand
	orderNumber int
}

// NewOrderGenerator creates a new order generator for the production line
func NewOrderGenerator(cfg *config.Config) *OrderGenerator {
	return &OrderGenerator{
		cfg:         cfg,
		rng:         rand.New(rand.NewSource(time.Now().UnixNano())),
		orderNumber: 1000,
	}
}

// GenerateOrder creates a new random production order
func (og *OrderGenerator) GenerateOrder() *core.ProductionOrder {
	// Select random part
	part := LineParts[og.rng.Intn(len(LineParts))]

	// Select random customer
	customer := LineCustomers[og.rng.Intn(len(LineCustomers))]

	// Generate random quantity within configured range
	quantity := og.cfg.OrderMinQty + og.rng.Intn(og.cfg.OrderMaxQty-og.cfg.OrderMinQty+1)

	// Calculate due date (8-48 hours from now)
	hoursUntilDue := 8 + og.rng.Intn(40)
	dueDate := time.Now().Add(time.Duration(hoursUntilDue) * time.Hour)

	// Generate priority (1=Urgent, 2=High, 3=Normal, 4=Low)
	priority := 1 + og.rng.Intn(4)

	og.orderNumber++
	orderID := fmt.Sprintf("LN-%d-%05d", time.Now().Year(), og.orderNumber)

	return &core.ProductionOrder{
		OrderID:           orderID,
		PartNumber:        part.PartNumber,
		PartDescription:   part.Description,
		Quantity:          quantity,
		QuantityCompleted: 0,
		QuantityScrap:     0,
		DueDate:           dueDate,
		Customer:          customer,
		Priority:          priority,
		Status:            core.OrderStatusQueued,
	}
}

// GenerateInitialQueue generates an initial queue of orders
func (og *OrderGenerator) GenerateInitialQueue(count int) []*core.ProductionOrder {
	orders := make([]*core.ProductionOrder, count)
	for i := 0; i < count; i++ {
		orders[i] = og.GenerateOrder()
	}
	return orders
}
