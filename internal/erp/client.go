package erp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/sebastiankruger/shopfloor-simulator/internal/config"
	"github.com/sebastiankruger/shopfloor-simulator/internal/simulator"
)

// Client handles communication with the ERP system
type Client struct {
	cfg        *config.Config
	httpClient *http.Client
}

// NewClient creates a new ERP client
func NewClient(cfg *config.Config) *Client {
	return &Client{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// SendOrderUpdate sends a production order update to the ERP endpoint
func (c *Client) SendOrderUpdate(ctx context.Context, order *simulator.ProductionOrder) error {
	url := c.cfg.ERPEndpoint + c.cfg.ERPOrderPath

	payload, err := json.Marshal(order)
	if err != nil {
		return fmt.Errorf("failed to marshal order: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		log.Warn().Err(err).Str("url", url).Msg("Failed to send order update (ERP endpoint may not be available)")
		return nil // Don't fail the simulator if ERP is unavailable
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		log.Warn().
			Int("status", resp.StatusCode).
			Str("orderId", order.OrderID).
			Msg("ERP returned error status for order update")
	} else {
		log.Debug().
			Str("orderId", order.OrderID).
			Str("status", order.Status).
			Msg("Order update sent to ERP")
	}

	return nil
}

// SendShiftUpdate sends a shift update to the ERP endpoint
func (c *Client) SendShiftUpdate(ctx context.Context, shift *simulator.Shift) error {
	url := c.cfg.ERPEndpoint + c.cfg.ERPShiftPath

	payload, err := json.Marshal(shift)
	if err != nil {
		return fmt.Errorf("failed to marshal shift: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		log.Warn().Err(err).Str("url", url).Msg("Failed to send shift update (ERP endpoint may not be available)")
		return nil // Don't fail the simulator if ERP is unavailable
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		log.Warn().
			Int("status", resp.StatusCode).
			Str("shiftId", shift.ShiftID).
			Msg("ERP returned error status for shift update")
	} else {
		log.Debug().
			Str("shiftId", shift.ShiftID).
			Str("status", shift.Status).
			Msg("Shift update sent to ERP")
	}

	return nil
}
