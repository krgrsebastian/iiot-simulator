package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/sebastiankruger/shopfloor-simulator/internal/config"
	"github.com/sebastiankruger/shopfloor-simulator/internal/erp"
	"github.com/sebastiankruger/shopfloor-simulator/internal/health"
	"github.com/sebastiankruger/shopfloor-simulator/internal/opcua"
	"github.com/sebastiankruger/shopfloor-simulator/internal/simulator"
)

func main() {
	// Setup logging
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339})

	// Recover from panics
	defer func() {
		if r := recover(); r != nil {
			log.Error().Interface("panic", r).Msg("Recovered from panic")
		}
	}()

	log.Info().Msg("Starting Welding Robot Shopfloor Simulator")

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to load configuration")
	}

	log.Info().
		Str("name", cfg.SimulatorName).
		Int("opcua_port", cfg.OPCUAPort).
		Str("erp_endpoint", cfg.ERPEndpoint).
		Dur("cycle_time", cfg.CycleTime).
		Msg("Configuration loaded")

	// Setup context with signal handling
	ctx, stop := signal.NotifyContext(context.Background(),
		syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Initialize components
	stateMachine := simulator.NewStateMachine(cfg)
	tsGenerator := simulator.NewTimeseriesGenerator()
	erpClient := erp.NewClient(cfg)
	orderGenerator := erp.NewOrderGenerator(cfg)
	shiftManager, err := erp.NewShiftManager(cfg)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create shift manager")
	}
	healthHandler := health.NewHandler()

	// Create OPC UA server
	opcuaServer, err := opcua.NewServer(cfg.OPCUAPort, cfg.SimulatorName)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create OPC UA server")
	}

	// Setup callbacks
	stateMachine.SetCallbacks(
		// On state change
		func(from, to simulator.MachineState) {
			log.Info().
				Str("from", from.String()).
				Str("to", to.String()).
				Msg("State changed")
		},
		// On cycle complete
		func(isScrap bool) {
			result := "good"
			if isScrap {
				result = "scrap"
			}
			log.Debug().Str("result", result).Msg("Cycle completed")

			// Send order update to ERP
			if order := stateMachine.GetCurrentOrder(); order != nil {
				go erpClient.SendOrderUpdate(ctx, order)
			}
		},
		// On order complete
		func(order *simulator.ProductionOrder) {
			log.Info().
				Str("orderId", order.OrderID).
				Int("completed", order.QuantityCompleted).
				Int("scrap", order.QuantityScrap).
				Msg("Order completed")

			go erpClient.SendOrderUpdate(ctx, order)

			// Generate new order
			newOrder := orderGenerator.GenerateOrder()
			stateMachine.AddOrder(newOrder)
			log.Info().
				Str("orderId", newOrder.OrderID).
				Int("quantity", newOrder.Quantity).
				Msg("New order generated")
		},
		// On error
		func(err *simulator.ErrorInfo) {
			log.Warn().
				Str("code", string(err.Code)).
				Str("message", err.Message).
				Time("expectedEnd", err.ExpectedEnd).
				Msg("Error occurred")
		},
	)

	// Generate initial orders
	initialOrders := orderGenerator.GenerateInitialQueue(3)
	for _, order := range initialOrders {
		stateMachine.AddOrder(order)
		log.Info().
			Str("orderId", order.OrderID).
			Str("part", order.PartNumber).
			Int("qty", order.Quantity).
			Msg("Initial order queued")
	}

	// Start OPC UA server
	if err := opcuaServer.Start(ctx); err != nil {
		log.Fatal().Err(err).Msg("Failed to start OPC UA server")
	}
	healthHandler.SetOPCUAReady(true)

	// Start health check HTTP server
	mux := http.NewServeMux()
	mux.HandleFunc("/health", healthHandler.HandleHealth)
	mux.HandleFunc("/health/live", healthHandler.HandleLive)
	mux.HandleFunc("/health/ready", healthHandler.HandleReady)

	healthServer := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.HealthPort),
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	go func() {
		log.Info().Int("port", cfg.HealthPort).Msg("Starting health check server")
		if err := healthServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error().Err(err).Msg("Health server error")
		}
	}()

	// Initialize shift
	currentShift := shiftManager.GetCurrentShift(time.Now())
	stateMachine.SetCurrentShift(currentShift)
	go erpClient.SendShiftUpdate(ctx, currentShift)
	log.Info().
		Str("shift", currentShift.ShiftName).
		Time("start", currentShift.StartTime).
		Time("end", currentShift.EndTime).
		Msg("Current shift initialized")

	// Main simulation loop
	ticker := time.NewTicker(cfg.PublishInterval)
	defer ticker.Stop()

	log.Info().
		Dur("interval", cfg.PublishInterval).
		Msg("Starting simulation loop")

	for {
		select {
		case <-ctx.Done():
			log.Info().Msg("Shutdown signal received")
			goto shutdown

		case now := <-ticker.C:
			// Check for shift change
			if newShift, changed := shiftManager.HasShiftChanged(now); changed {
				log.Info().
					Str("shift", newShift.ShiftName).
					Msg("Shift changed")

				stateMachine.SetCurrentShift(newShift)
				stateMachine.ResetCounters()
				go erpClient.SendShiftUpdate(ctx, newShift)
			}

			// Check if it's break time
			isBreakTime := shiftManager.IsBreakTime(now, shiftManager.GetCurrentShiftRef())

			// Update state machine
			stateMachine.Update(now, isBreakTime)

			// Get current state
			state := stateMachine.GetState()

			// Calculate phase progress
			var phaseProgress float64
			if state.State == simulator.StateRunning {
				phaseProgress = simulator.CalculatePhaseProgress(
					state.CycleStartedAt,
					cfg.CycleTime,
					state.WeldPhase,
				)
			}

			// Generate timeseries data
			tsData := tsGenerator.Generate(state.State, state.WeldPhase, phaseProgress)

			// Add state information
			goodParts, scrapParts, arcTime := stateMachine.GetCounters()
			tsData.GoodParts = goodParts
			tsData.ScrapParts = scrapParts
			tsData.ArcTime = arcTime
			tsData.CycleProgress = stateMachine.GetCycleProgress()

			if order := stateMachine.GetCurrentOrder(); order != nil {
				tsData.CurrentOrderID = order.OrderID
				tsData.CurrentPartNumber = order.PartNumber
			}

			if state.CurrentError != nil {
				tsData.ErrorCode = string(state.CurrentError.Code)
				tsData.ErrorMessage = state.CurrentError.Message
				tsData.ErrorTimestamp = state.CurrentError.OccurredAt
			}

			// Update OPC UA values
			opcuaServer.UpdateValues(&tsData)

			// Log periodic status
			if now.Second()%10 == 0 {
				log.Debug().
					Str("state", state.State.String()).
					Float64("current", tsData.WeldingCurrent).
					Float64("voltage", tsData.Voltage).
					Int("goodParts", goodParts).
					Int("scrapParts", scrapParts).
					Msg("Simulation tick")
			}
		}
	}

shutdown:
	log.Info().Msg("Shutting down...")

	// Create shutdown context with timeout
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Shutdown health server
	if err := healthServer.Shutdown(shutdownCtx); err != nil {
		log.Error().Err(err).Msg("Health server shutdown error")
	}

	// Shutdown OPC UA server
	if err := opcuaServer.Stop(shutdownCtx); err != nil {
		log.Error().Err(err).Msg("OPC UA server shutdown error")
	}

	log.Info().Msg("Simulator stopped")
}
