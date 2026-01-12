# Production Line Simulator Documentation

This document provides comprehensive documentation for the multi-machine production line simulator mode.

## Table of Contents

1. [Overview](#overview)
2. [Architecture](#architecture)
3. [Machine Types](#machine-types)
4. [Part Flow](#part-flow)
5. [Configuration Reference](#configuration-reference)
6. [OPC UA Node Reference](#opc-ua-node-reference)
7. [Parameter Relationships](#parameter-relationships)
8. [Error Handling](#error-handling)
9. [ERP Integration](#erp-integration)
10. [Troubleshooting](#troubleshooting)

---

## Overview

The Production Line Simulator creates a realistic multi-machine manufacturing environment with:

- **3 Connected Machines**: Forming Machine, Picker Robot, Spot Welder
- **4 OPC UA Namespaces**: One per machine plus line-level coordination
- **Realistic Part Flow**: Parts move through buffers between machines
- **Production Metrics**: OEE, throughput, bottleneck detection
- **ERP Integration**: Order and shift updates via REST API

### Quick Start

```bash
# Production line mode (set LINE_TYPE to any value)
docker run -d \
  -p 4840:4840 \
  -p 8081:8081 \
  -e LINE_TYPE=forming-picker-spotwelder \
  -e SIMULATOR_NAME=ProductionLine-01 \
  -e CYCLE_TIME=15s \
  shopfloor-simulator:latest
```

---

## Architecture

```
┌─────────────────┐     ┌─────────────┐     ┌─────────────────┐     ┌─────────────────┐
│ Forming Machine │────>│  Buffer (5) │────>│  Picker Robot   │────>│   Spot Welder   │
│     (ns=2)      │     │             │     │     (ns=3)      │     │     (ns=4)      │
└─────────────────┘     └─────────────┘     └─────────────────┘     └─────────────────┘
                                                    │
                                                    v
                                            ┌─────────────┐
                                            │  Buffer (3) │
                                            └─────────────┘
                                                    │
                                    ┌───────────────┴───────────────┐
                                    │   Production Line Coordinator │
                                    │            (ns=5)             │
                                    └───────────────────────────────┘
```

### OPC UA Namespaces

| Namespace | Name | Description |
|-----------|------|-------------|
| ns=2 | FormingMachine | Sheet metal forming press |
| ns=3 | PickerRobot | 6-axis pick and place robot |
| ns=4 | SpotWelder | Stud spot welding machine |
| ns=5 | ProductionLine | Line-level metrics and coordination |

---

## Machine Types

### Forming Machine (ns=2)

A hydraulic sheet metal forming press that creates the base parts.

#### Operating Phases

| Phase | Duration | Description |
|-------|----------|-------------|
| Idle | - | Waiting for work |
| Load | 10% | Sheet metal loading |
| Form | 40% | Ram descending, forming part |
| Hold | 15% | Pressure maintained |
| Eject | 15% | Part ejection |
| Raise | 20% | Ram returning to top |

#### Key Parameters

| Parameter | Default | Unit | Description |
|-----------|---------|------|-------------|
| TargetTemperature | 45.0 | C | Die operating temperature |
| MaxPressure | 150.0 | bar | Hydraulic system pressure |
| MaxFormingForce | 250.0 | kN | Maximum ram force |
| RamTravel | 400.0 | mm | Total ram travel distance |
| MaxRamSpeed | 80.0 | mm/s | Maximum descent speed |
| OutputBufferCapacity | 5 | parts | Parts waiting for pickup |

#### What Influences What

```
Temperature ──────> Quality (affects scrap rate)
Pressure ─────────> FormingForce (proportional)
RamPosition ──────> Phase progression
RamSpeed ─────────> Cycle time
OutputBufferCount ─> Picker starvation/blocking
```

---

### Picker Robot (ns=3)

A 6-axis industrial robot that transfers parts between stations.

#### Operating Phases

| Phase | Duration | Description |
|-------|----------|-------------|
| Idle | - | Waiting at home position |
| MoveToPickup | 15% | Moving to forming output |
| ApproachPickup | 8% | Descending to part |
| Grip | 5% | Closing gripper on part |
| RetractPickup | 8% | Lifting part |
| MoveToPlace | 20% | Moving to welder input |
| ApproachPlace | 8% | Descending to place position |
| Release | 5% | Opening gripper |
| RetractPlace | 8% | Lifting away |

#### Key Parameters

| Parameter | Default | Unit | Description |
|-----------|---------|------|-------------|
| MaxReachX/Y/Z | 1500/1500/1000 | mm | Workspace envelope |
| MaxSpeed | 500.0 | mm/s | TCP (tool center point) speed |
| MaxJointSpeed | 180.0 | deg/s | Joint rotation speed |
| AccelerationG | 1.5 | g | Acceleration limit |
| MaxGripForce | 100.0 | N | Gripper force |
| GripTime | 300 | ms | Time to grip/release |
| HomePosition | (500,0,800) | mm | Home position |
| PickupPosition | (100,300,200) | mm | Over forming output |
| PlacePosition | (900,300,200) | mm | Over welder input |

#### Gripper States

| State | Value | Description |
|-------|-------|-------------|
| GripperOpen | 0 | Fully open |
| GripperClosing | 1 | Closing on part |
| GripperClosed | 2 | Holding part |
| GripperOpening | 3 | Releasing part |

#### What Influences What

```
Position (X,Y,Z) ──> Phase progression
Speed ────────────> Cycle time
GripperState ─────> Part transfer success
GripForce ────────> Part security (drop errors)
PartInGripper ────> Current part ID being transferred
```

---

### Spot Welder (ns=4)

A stud spot welding machine that completes the assembly with 4 welds per part.

#### Operating Phases

| Phase | Duration | Description |
|-------|----------|-------------|
| Idle | - | Waiting for part |
| Load | 10% | Part loaded onto fixture |
| Clamp | 10% | Fixtures clamping part |
| PreWeld | 5% | Positioning weld stud |
| Weld | 35% | Welding current flowing (4 welds) |
| Hold | 15% | Post-weld cooling |
| Release | 10% | Fixtures releasing |
| Unload | 15% | Part unloading |

#### Key Parameters

| Parameter | Default | Unit | Description |
|-----------|---------|------|-------------|
| TargetCurrent | 8.0 | kA | Welding current |
| TargetVoltage | 2.5 | V | Welding voltage |
| WeldDuration | 200 | ms | Duration per weld |
| WeldsPerPart | 4 | count | Studs welded per part |
| MaxElectrodeForce | 3.0 | kN | Electrode pressure |
| MaxClampForce | 5.0 | kN | Fixture clamp force |
| MaxElectrodeTemp | 400.0 | C | Temperature limit |
| ElectrodeLifeWelds | 5000 | welds | Electrode replacement interval |
| InputBufferCapacity | 3 | parts | Parts waiting for welding |

#### What Influences What

```
WeldCurrent ──────> WeldEnergy (calculated)
WeldVoltage ──────> WeldEnergy (calculated)
ElectrodeTemp ────> Overheat errors (>400C)
ElectrodeWear ────> Maintenance alerts (>100%)
TotalWelds ───────> ElectrodeWear (TotalWelds/5000 * 100%)
WeldCount ────────> Phase progression (0-4 per part)
```

---

## Part Flow

### Part Lifecycle

```
1. CREATED at Forming
   ├── Part ID generated: PART-YYYY-MM-DD-NNNN
   └── Status: IN_FORMING

2. FORMING COMPLETE
   ├── Part pushed to output buffer
   └── Status: AWAITING_PICKUP

3. PICKED UP by Robot
   ├── Part removed from forming buffer
   └── Status: IN_TRANSIT

4. PLACED at Welder
   ├── Part pushed to welder input buffer
   └── Status: AWAITING_WELDING

5. WELDING COMPLETE
   ├── 4 welds completed
   └── Status: COMPLETE or SCRAP
```

### Buffer Behavior

| Buffer | Capacity | Location | Effect When Full |
|--------|----------|----------|------------------|
| Forming Output | 5 | FormingMachine.OutputBuffer | Forming pauses |
| Welder Input | 3 | SpotWelder.InputBuffer | Picker pauses |

**Starvation**: When a buffer is empty, the downstream machine idles.
**Blocking**: When a buffer is full, the upstream machine pauses.

---

## Configuration Reference

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `LINE_TYPE` | (empty) | Set to any value to enable production line mode |
| `SIMULATOR_NAME` | WeldingRobot-01 | Name shown in OPC UA |
| `OPCUA_PORT` | 4840 | OPC UA server port |
| `HEALTH_PORT` | 8081 | Health check HTTP port |
| `CYCLE_TIME` | 60s | Base cycle time for machines |
| `SETUP_TIME` | 45s | Changeover time between orders |
| `PUBLISH_INTERVAL` | 1s | OPC UA value update rate |
| `SCRAP_RATE` | 0.03 | Probability of defect (0.0-1.0) |
| `ERROR_RATE` | 0.02 | Probability of error (0.0-1.0) |
| `ORDER_MIN_QTY` | 50 | Minimum order quantity |
| `ORDER_MAX_QTY` | 500 | Maximum order quantity |
| `TIMEZONE` | Europe/Berlin | Shift schedule timezone |
| `SHIFT_MODEL` | 3-shift | Shift configuration |
| `ERP_ENDPOINT` | http://localhost:8080 | ERP API base URL |
| `ERP_ORDER_PATH` | /api/v1/production-orders | Order update endpoint |
| `ERP_SHIFT_PATH` | /api/v1/shifts | Shift update endpoint |

### Cycle Time Relationships

The picker robot runs **3x faster** than forming and welding to prevent bottlenecks:

```
Forming Cycle Time = CYCLE_TIME (e.g., 15s)
Picker Cycle Time  = CYCLE_TIME / 3 (e.g., 5s)
Welder Cycle Time  = CYCLE_TIME (e.g., 15s)
```

### Throughput Calculation

```
Theoretical Max = 3600 / CYCLE_TIME parts/hour

With 15s cycle time:
  = 3600 / 15 = 240 parts/hour theoretical

Actual throughput affected by:
  - Error rate (downtime)
  - Scrap rate (quality losses)
  - Buffer starvation/blocking
```

---

## OPC UA Node Reference

### Forming Machine (ns=2)

| Node | Type | Unit | Description |
|------|------|------|-------------|
| Temperature | Double | C | Process temperature |
| Pressure | Double | bar | Hydraulic pressure |
| FormingForce | Double | kN | Ram force |
| RamPosition | Double | mm | Ram position (0=top) |
| RamSpeed | Double | mm/s | Ram velocity |
| DieTemperature | Double | C | Die surface temperature |
| CycleCount | Int32 | - | Total cycles completed |
| CycleTime | Double | s | Current cycle elapsed time |
| State | Int32 | - | Machine state (0-4) |
| GoodParts | Int32 | - | Good parts counter |
| ScrapParts | Int32 | - | Scrap parts counter |
| CurrentOrderId | String | - | Active order ID |
| CurrentPartNumber | String | - | Part number being made |
| CycleProgress | Double | % | Cycle completion (0-100) |
| OutputBufferCount | Int32 | - | Parts in output buffer |
| CurrentPartId | String | - | Part currently being formed |
| ErrorCode | String | - | Current error code |
| ErrorMessage | String | - | Error description |

### Picker Robot (ns=3)

| Node | Type | Unit | Description |
|------|------|------|-------------|
| PositionX | Double | mm | X position |
| PositionY | Double | mm | Y position |
| PositionZ | Double | mm | Z position |
| Speed | Double | mm/s | Current TCP speed |
| Joint1-6 | Double | deg | Joint angles |
| GripperState | Int32 | - | Gripper state (0-3) |
| GripperPosition | Double | % | Gripper opening (0-100) |
| GripForce | Double | N | Current grip force |
| CycleCount | Int32 | - | Total cycles completed |
| CycleTime | Double | s | Current cycle elapsed time |
| State | Int32 | - | Machine state (0-4) |
| GoodParts | Int32 | - | Parts transferred |
| ScrapParts | Int32 | - | Parts dropped |
| CurrentOrderId | String | - | Active order ID |
| CurrentPartNumber | String | - | Part number |
| CycleProgress | Double | % | Cycle completion (0-100) |
| PartInGripper | String | - | Part ID being held |
| ErrorCode | String | - | Current error code |
| ErrorMessage | String | - | Error description |

### Spot Welder (ns=4)

| Node | Type | Unit | Description |
|------|------|------|-------------|
| WeldCurrent | Double | kA | Welding current |
| WeldVoltage | Double | V | Welding voltage |
| WeldTime | Double | ms | Weld duration |
| WeldEnergy | Double | J | Energy per weld |
| ElectrodeForce | Double | kN | Electrode pressure |
| ClampForce | Double | kN | Fixture clamp force |
| ElectrodeTemp | Double | C | Electrode temperature |
| PartTemp | Double | C | Part temperature |
| WeldCount | Int32 | - | Welds on current part (0-4) |
| TotalWelds | Int32 | - | Total welds performed |
| CycleCount | Int32 | - | Total parts welded |
| CycleTime | Double | s | Current cycle elapsed time |
| State | Int32 | - | Machine state (0-4) |
| GoodParts | Int32 | - | Good parts counter |
| ScrapParts | Int32 | - | Scrap parts counter |
| CurrentOrderId | String | - | Active order ID |
| CurrentPartNumber | String | - | Part number |
| CycleProgress | Double | % | Cycle completion (0-100) |
| CurrentPartId | String | - | Part being welded |
| ElectrodeWear | Double | % | Electrode wear (0-100) |
| ErrorCode | String | - | Current error code |
| ErrorMessage | String | - | Error description |

### Production Line (ns=5)

| Node | Type | Unit | Description |
|------|------|------|-------------|
| LineState | String | - | Running/Stopped/Error |
| WIPCount | Int32 | - | Work in progress |
| ThroughputPerHour | Double | parts/h | Current throughput |
| BottleneckMachine | String | - | Slowest machine |
| TotalPartsCompleted | Int32 | - | Final good parts |
| TotalPartsScrap | Int32 | - | Total scrapped |
| FormingCompleted | Int32 | - | Parts through forming |
| PickingCompleted | Int32 | - | Parts through picker |
| WeldingCompleted | Int32 | - | Parts through welder |
| Availability | Double | % | Uptime percentage |
| Performance | Double | % | Speed efficiency |
| Quality | Double | % | First-pass yield |
| OEE | Double | % | Overall Equipment Effectiveness |
| CurrentOrderId | String | - | Active order ID |
| OrderProgress | Double | % | Order completion |
| ActiveErrors | Int32 | - | Number of active errors |
| LastErrorCode | String | - | Most recent error |
| LastErrorMachine | String | - | Machine with error |

### Machine States

| Value | State | Description |
|-------|-------|-------------|
| 0 | Idle | Waiting for work |
| 1 | Setup | Changeover/preparation |
| 2 | Running | Active production |
| 3 | PlannedStop | Break or scheduled stop |
| 4 | UnplannedStop | Error/breakdown |

---

## Parameter Relationships

### How Values Influence Each Other

```
┌──────────────────────────────────────────────────────────────┐
│                    CONFIGURATION                              │
├──────────────────────────────────────────────────────────────┤
│ CYCLE_TIME ─────────> Theoretical throughput                 │
│ ERROR_RATE ─────────> Availability (downtime frequency)      │
│ SCRAP_RATE ─────────> Quality (defect frequency)             │
│ SETUP_TIME ─────────> Changeover losses                      │
└──────────────────────────────────────────────────────────────┘
                              │
                              v
┌──────────────────────────────────────────────────────────────┐
│                    MACHINE LEVEL                              │
├──────────────────────────────────────────────────────────────┤
│ State ──────────────> Phase progression allowed              │
│ Phase ──────────────> Parameter values (current, position)   │
│ CycleProgress ──────> Phase transitions                      │
│ ErrorCode ──────────> State = UnplannedStop                  │
│ BufferCount ────────> Upstream blocking / downstream starve  │
└──────────────────────────────────────────────────────────────┘
                              │
                              v
┌──────────────────────────────────────────────────────────────┐
│                    LINE LEVEL                                 │
├──────────────────────────────────────────────────────────────┤
│ FormingCompleted ───> PickingCompleted ───> WeldingCompleted │
│ Machine States ─────> LineState (any error = Error)          │
│ Throughputs ────────> BottleneckMachine (slowest)            │
│ All metrics ────────> OEE calculation                        │
└──────────────────────────────────────────────────────────────┘
```

### OEE Calculation

```
OEE = Availability x Performance x Quality

Availability = (Total Time - Downtime) / Total Time
  - Reduced by: UnplannedStop time, error recovery
  - NOT reduced by: PlannedStop (breaks)

Performance = Actual Throughput / Theoretical Throughput
  - Reduced by: Slow cycles, buffer starvation
  - Theoretical = 3600 / CYCLE_TIME

Quality = Good Parts / (Good Parts + Scrap Parts)
  - Reduced by: SCRAP_RATE setting
  - Each defect counted immediately
```

---

## Error Handling

### Error Codes by Machine

#### Forming Machine Errors

| Code | Description | Duration |
|------|-------------|----------|
| F001 | Sheet metal misfeed | 2-5 min |
| F002 | Hydraulic system fault | 5-15 min |
| F003 | Overpressure detected | 3-8 min |
| F004 | Ram stuck in position | 10-30 min |
| F005 | Quality reject - forming defect | 1-2 min |
| F006 | Temperature out of range | 5-10 min |

#### Picker Robot Errors

| Code | Description | Duration |
|------|-------------|----------|
| P001 | Gripper mechanism fault | 2-5 min |
| P002 | Collision detected | 5-15 min |
| P003 | Position feedback error | 3-8 min |
| P004 | Part dropped during transfer | 2-5 min |
| P005 | Servo motor overload | 5-10 min |
| P006 | Emergency stop activated | 1-3 min |

#### Spot Welder Errors

| Code | Description | Duration |
|------|-------------|----------|
| S001 | Weld quality fault detected | 2-5 min |
| S002 | Clamp mechanism fault | 3-8 min |
| S003 | Stud feed mechanism jam | 2-5 min |
| S004 | Electrode overheat protection | 5-15 min |
| S005 | Weld current out of range | 3-7 min |
| S006 | Weld quality below threshold | 1-3 min |

### Error Recovery

1. Error occurs during running state
2. Machine transitions to `StateUnplannedStop`
3. `ErrorCode` and `ErrorMessage` populated
4. Random recovery duration selected (min-max range)
5. After duration, machine auto-recovers to `StateIdle`
6. Error fields cleared
7. Machine resumes production

---

## ERP Integration

### Order Updates

Sent to `ERP_ENDPOINT + ERP_ORDER_PATH` every 5 seconds:

```json
{
  "orderId": "LN-2026-01001",
  "partNumber": "BRKT-FRM-C01",
  "partDescription": "Support Bracket",
  "quantity": 150,
  "quantityCompleted": 47,
  "quantityScrap": 2,
  "dueDate": "2026-01-15T18:00:00Z",
  "customer": "AutoCorp Inc.",
  "priority": 2,
  "status": "IN_PROGRESS",
  "startedAt": "2026-01-15T06:12:00Z"
}
```

### Shift Updates

Sent to `ERP_ENDPOINT + ERP_SHIFT_PATH` on shift changes:

```json
{
  "shiftId": "SHIFT-2026-01-12-M",
  "shiftName": "Morning",
  "shiftNumber": 1,
  "startTime": "2026-01-12T06:00:00Z",
  "endTime": "2026-01-12T14:00:00Z",
  "workCenterId": "WC-LINE-01",
  "status": "ACTIVE"
}
```

### Part Catalog

| Part Number | Description | Typical Cycle |
|-------------|-------------|---------------|
| BRKT-FRM-A01 | Front Frame Bracket | Standard |
| BRKT-FRM-B02 | Rear Frame Bracket | Standard |
| BRKT-FRM-C01 | Support Bracket | Fast |
| RAIL-ASM-D01 | Side Rail Assembly | Standard |
| RAIL-ASM-E01 | Cross Rail Assembly | Slow |
| STUD-MNT-F01 | Stud Mount Plate | Fast |

---

## Troubleshooting

### Common Issues

#### Line Stops After Error

**Symptom**: Production stops, logs show "Running" but counters don't increment.

**Cause**: Order tracking bug (fixed in v1.1)

**Solution**: Ensure you're running latest version with these fixes:
- `orderPartsCompleted` tracking per order (not cumulative)
- Null order check in main loop

#### No Data in TimescaleDB

**Symptom**: OPC UA running but no data in database.

**Cause**: OPC UA connection lost after simulator restart.

**Solution**: Restart the OPC UA client (umh-core) to reconnect.

#### Buffer Always Full/Empty

**Symptom**: One machine always idle, buffer at capacity or empty.

**Cause**: Cycle time mismatch between machines.

**Solution**: Ensure picker runs 3x faster than forming/welding:
```
Picker Cycle = CYCLE_TIME / 3
```

### Debug Logging

Enable debug output:
```bash
# View simulator logs
docker logs production-line-simulator -f

# Filter for specific machine
docker logs production-line-simulator 2>&1 | grep -i "forming"

# Check order transitions
docker logs production-line-simulator 2>&1 | grep -i "order"
```

### Health Checks

```bash
# Overall health
curl http://localhost:8081/health

# Liveness (is process running)
curl http://localhost:8081/health/live

# Readiness (is OPC UA server ready)
curl http://localhost:8081/health/ready
```

---

## Version History

| Version | Date | Changes |
|---------|------|---------|
| 1.0 | 2026-01-09 | Initial production line mode |
| 1.1 | 2026-01-12 | Fixed order tracking bugs, added null order check |
