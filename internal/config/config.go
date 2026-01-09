package config

import (
	"os"
	"strconv"
	"time"
)

// Config holds all configuration for the simulator
type Config struct {
	// Core settings
	SimulatorName string
	OPCUAPort     int
	HealthPort    int

	// ERP settings
	ERPEndpoint  string
	ERPOrderPath string
	ERPShiftPath string

	// Timing settings
	PublishInterval time.Duration
	CycleTime       time.Duration
	SetupTime       time.Duration

	// Production settings
	ScrapRate   float64
	ErrorRate   float64
	OrderMinQty int
	OrderMaxQty int

	// Shift settings
	Timezone   string
	ShiftModel string
}

// Load reads configuration from environment variables with defaults
func Load() (*Config, error) {
	cfg := &Config{
		// Core settings
		SimulatorName: getEnvOrDefault("SIMULATOR_NAME", "WeldingRobot-01"),
		OPCUAPort:     getEnvAsIntOrDefault("OPCUA_PORT", 4840),
		HealthPort:    getEnvAsIntOrDefault("HEALTH_PORT", 8081),

		// ERP settings
		ERPEndpoint:  getEnvOrDefault("ERP_ENDPOINT", "http://localhost:8080"),
		ERPOrderPath: getEnvOrDefault("ERP_ORDER_PATH", "/api/v1/production-orders"),
		ERPShiftPath: getEnvOrDefault("ERP_SHIFT_PATH", "/api/v1/shifts"),

		// Timing settings
		PublishInterval: getDurationOrDefault("PUBLISH_INTERVAL", 1*time.Second),
		CycleTime:       getDurationOrDefault("CYCLE_TIME", 60*time.Second),
		SetupTime:       getDurationOrDefault("SETUP_TIME", 45*time.Second),

		// Production settings
		ScrapRate:   getEnvAsFloatOrDefault("SCRAP_RATE", 0.03),
		ErrorRate:   getEnvAsFloatOrDefault("ERROR_RATE", 0.02),
		OrderMinQty: getEnvAsIntOrDefault("ORDER_MIN_QTY", 50),
		OrderMaxQty: getEnvAsIntOrDefault("ORDER_MAX_QTY", 500),

		// Shift settings
		Timezone:   getEnvOrDefault("TIMEZONE", "Europe/Berlin"),
		ShiftModel: getEnvOrDefault("SHIFT_MODEL", "3-shift"),
	}

	return cfg, nil
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvAsIntOrDefault(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}

func getEnvAsFloatOrDefault(key string, defaultValue float64) float64 {
	if value := os.Getenv(key); value != "" {
		if floatVal, err := strconv.ParseFloat(value, 64); err == nil {
			return floatVal
		}
	}
	return defaultValue
}

func getDurationOrDefault(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}
