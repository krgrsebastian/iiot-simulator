# Shopfloor Simulator

A realistic shopfloor simulator that generates production data for manufacturing equipment. Supports both standalone machine mode and multi-machine production line mode. Designed for demos, training, and testing manufacturing data pipelines.

## Features

- **OPC UA Server**: Exposes timeseries data via industry-standard protocol
- **REST API Client**: Sends production orders and shift data to an ERP endpoint
- **Realistic Data**: Gaussian noise, parameter correlations, ramp-up/ramp-down phases
- **State Machine**: Idle → Setup → Running → Planned/Unplanned Stop
- **3-Shift Support**: 24/7 operation with configurable breaks
- **Auto-generated Orders**: Continuous production simulation

## Simulation Modes

### Mode 1: Standalone Welding Robot (Default)
Single welding robot simulator with welding-specific parameters.

### Mode 2: Production Line (Multi-Machine)
Connected production line with 3 machines and part flow:
- **Forming Machine** (ns=2): Sheet metal forming press
- **Picker Robot** (ns=3): 6-axis pick and place robot
- **Spot Welder** (ns=4): Stud spot welding machine
- **Line Coordinator** (ns=5): OEE metrics and bottleneck detection

See [Production Line Documentation](docs/PRODUCTION_LINE.md) for detailed information.

## Quick Start

### Using Docker Hub Image (Recommended)

Pull and run directly from Docker Hub:

```bash
docker run -d \
  --name welding-simulator \
  -p 4840:4840 \
  -p 8081:8081 \
  -e CYCLE_TIME=30s \
  -e SCRAP_RATE=0.05 \
  skumh/iiot-simulator:latest
```

Or use this `docker-compose.yml`:

```yaml
services:
  welding-simulator:
    image: skumh/iiot-simulator:latest
    container_name: welding-simulator
    ports:
      - "4840:4840"   # OPC UA
      - "8081:8081"   # Health check
    environment:
      - SIMULATOR_NAME=WeldingRobot-01
      - CYCLE_TIME=30s
      - SETUP_TIME=15s
      - SCRAP_RATE=0.05
      - ERROR_RATE=0.03
      - TIMEZONE=Europe/Berlin
      # Optional: Connect to an ERP endpoint
      # - ERP_ENDPOINT=http://your-erp:8080
      # - ERP_ORDER_PATH=/api/v1/production-orders
      # - ERP_SHIFT_PATH=/api/v1/shifts
    healthcheck:
      test: ["CMD", "wget", "--no-verbose", "--tries=1", "--spider", "http://localhost:8081/health"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 10s
    restart: unless-stopped
```

Then run:

```bash
docker-compose up -d
docker-compose logs -f
```

### Production Line Mode

To run the multi-machine production line simulator:

```bash
docker run -d \
  --name production-line \
  -p 4840:4840 \
  -p 8081:8081 \
  -e LINE_TYPE=forming-picker-spotwelder \
  -e SIMULATOR_NAME=ProductionLine-01 \
  -e CYCLE_TIME=15s \
  shopfloor-simulator:latest
```

Or with docker-compose:

```yaml
services:
  production-line:
    image: shopfloor-simulator:latest
    container_name: production-line-simulator
    ports:
      - "4840:4840"   # OPC UA (4 namespaces)
      - "8081:8081"   # Health check
    environment:
      - LINE_TYPE=forming-picker-spotwelder
      - SIMULATOR_NAME=ProductionLine-01
      - CYCLE_TIME=15s
      - SETUP_TIME=5s
      - SCRAP_RATE=0.03
      - ERROR_RATE=0.02
    healthcheck:
      test: ["CMD", "wget", "--spider", "http://localhost:8081/health"]
      interval: 10s
      timeout: 5s
      retries: 3
    restart: unless-stopped
```

### Building from Source

```bash
# Build and run locally
docker-compose up -d --build

# View logs
docker-compose logs -f

# Stop
docker-compose down
```

### Using Go directly

```bash
# Build
go build -o simulator ./cmd/simulator

# Run with defaults
./simulator

# Run with custom settings
CYCLE_TIME=30s SCRAP_RATE=0.05 ./simulator
```

## Configuration

All configuration is done via environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `LINE_TYPE` | (empty) | Set to any value to enable production line mode |
| `SIMULATOR_NAME` | `WeldingRobot-01` | Machine/line identifier |
| `OPCUA_PORT` | `4840` | OPC UA server port |
| `HEALTH_PORT` | `8081` | Health check HTTP port |
| `ERP_ENDPOINT` | `http://localhost:8080` | ERP REST API base URL |
| `CYCLE_TIME` | `60s` | Production cycle time |
| `SETUP_TIME` | `45s` | Setup/changeover time |
| `SCRAP_RATE` | `0.03` | Scrap probability (0.0-1.0) |
| `ERROR_RATE` | `0.02` | Error probability per cycle |
| `TIMEZONE` | `Europe/Berlin` | Timezone for shift schedule |
| `SHIFT_MODEL` | `3-shift` | Shift model (3-shift, 2-shift, 1-shift) |

For production line mode configuration details, see [docs/PRODUCTION_LINE.md](docs/PRODUCTION_LINE.md).

## OPC UA Nodes

Connect to `opc.tcp://localhost:4840` and browse the following nodes:

### Welding Parameters
| Node ID | Description | Unit |
|---------|-------------|------|
| `ns=2;s=Robot.WeldingCurrent` | Welding current | A |
| `ns=2;s=Robot.Voltage` | Arc voltage | V |
| `ns=2;s=Robot.WireFeedSpeed` | Wire feed speed | m/min |
| `ns=2;s=Robot.GasFlow` | Shielding gas flow | l/min |
| `ns=2;s=Robot.TravelSpeed` | Travel speed | mm/s |
| `ns=2;s=Robot.ArcTime` | Cumulative arc time | s |

### Position
| Node ID | Description | Unit |
|---------|-------------|------|
| `ns=2;s=Robot.Position.X` | X position | mm |
| `ns=2;s=Robot.Position.Y` | Y position | mm |
| `ns=2;s=Robot.Position.Z` | Z position | mm |
| `ns=2;s=Robot.TorchAngle` | Torch angle | deg |

### Production
| Node ID | Description |
|---------|-------------|
| `ns=2;s=Robot.State` | Machine state (0-4) |
| `ns=2;s=Robot.GoodParts` | Good parts count |
| `ns=2;s=Robot.ScrapParts` | Scrap parts count |
| `ns=2;s=Robot.CurrentOrderId` | Active order ID |
| `ns=2;s=Robot.CycleProgress` | Cycle progress (0-100%) |

### Errors
| Node ID | Description |
|---------|-------------|
| `ns=2;s=Robot.ErrorCode` | Current error code |
| `ns=2;s=Robot.ErrorMessage` | Error description |
| `ns=2;s=Robot.ErrorTimestamp` | When error occurred |

### Production Line Nodes

In production line mode, additional namespaces are available:
- `ns=2` - Forming Machine (Temperature, Pressure, FormingForce, RamPosition, etc.)
- `ns=3` - Picker Robot (PositionX/Y/Z, GripperState, PartInGripper, etc.)
- `ns=4` - Spot Welder (WeldCurrent, WeldVoltage, ElectrodeTemp, ElectrodeWear, etc.)
- `ns=5` - Production Line (LineState, OEE, Availability, BottleneckMachine, etc.)

See [docs/PRODUCTION_LINE.md](docs/PRODUCTION_LINE.md) for the complete node reference.

## REST API Output

The simulator sends JSON payloads to your configured ERP endpoint:

### Production Orders

`POST {ERP_ENDPOINT}/api/v1/production-orders`

```json
{
  "orderId": "PO-2024-001234",
  "partNumber": "WLD-FRAME-A01",
  "partDescription": "Front Frame Assembly",
  "quantity": 150,
  "quantityCompleted": 47,
  "quantityScrap": 2,
  "dueDate": "2024-01-15T18:00:00Z",
  "customer": "AutoCorp Inc.",
  "priority": 2,
  "status": "IN_PROGRESS",
  "startedAt": "2024-01-15T06:12:00Z"
}
```

### Shift Data

`POST {ERP_ENDPOINT}/api/v1/shifts`

```json
{
  "shiftId": "SHIFT-2024-01-15-M",
  "shiftName": "Morning",
  "shiftNumber": 1,
  "startTime": "2024-01-15T06:00:00Z",
  "endTime": "2024-01-15T14:00:00Z",
  "workCenterId": "WC-WELD-01",
  "plannedBreaks": [
    {"start": "2024-01-15T09:00:00Z", "end": "2024-01-15T09:15:00Z", "type": "break"}
  ],
  "status": "ACTIVE"
}
```

## Health Checks

- `GET /health` - Combined health check (for Docker)
- `GET /health/live` - Liveness probe (is the app running?)
- `GET /health/ready` - Readiness probe (is the app ready for traffic?)

## Machine States

| State | Value | Description |
|-------|-------|-------------|
| Idle | 0 | Waiting for work |
| Setup | 1 | Changeover/preparation |
| Running | 2 | Active welding |
| PlannedStop | 3 | Break or scheduled stop |
| UnplannedStop | 4 | Error/breakdown |

## Testing

### With UaExpert

1. Download [UaExpert](https://www.unified-automation.com/products/development-tools/uaexpert.html)
2. Add server: `opc.tcp://localhost:4840`
3. Browse nodes and subscribe to values

### With curl (Health checks)

```bash
# Liveness check
curl http://localhost:8081/health/live

# Readiness check
curl http://localhost:8081/health/ready
```

## License

MIT
