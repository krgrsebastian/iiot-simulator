package picker

import (
	"math"
	"time"

	"github.com/sebastiankruger/shopfloor-simulator/internal/core"
)

// TimeseriesGenerator generates realistic timeseries data for a picker robot
type TimeseriesGenerator struct {
	config PickerConfig
	noise  *core.NoiseGenerator

	// Internal state for smooth speed calculation
	lastPosition Position3D
	lastTime     time.Time
}

// NewTimeseriesGenerator creates a new timeseries generator for picker robots
func NewTimeseriesGenerator(cfg PickerConfig) *TimeseriesGenerator {
	return &TimeseriesGenerator{
		config:       cfg,
		noise:        core.NewNoiseGenerator(),
		lastPosition: cfg.HomePosition,
		lastTime:     time.Now(),
	}
}

// Generate generates timeseries data based on current state, phase, and position
func (tg *TimeseriesGenerator) Generate(state core.MachineState, phase PickerPhase, phaseProgress float64,
	position Position3D, gripper GripperState, gripperPosition float64) *PickerData {

	data := &PickerData{
		PositionX:       position.X,
		PositionY:       position.Y,
		PositionZ:       position.Z,
		GripperState:    gripper,
		GripperPosition: gripperPosition,
		Timestamp:       time.Now(),
	}

	// Calculate speed from position delta
	now := time.Now()
	dt := now.Sub(tg.lastTime).Seconds()
	if dt > 0 && dt < 1.0 { // Reasonable time delta
		dx := position.X - tg.lastPosition.X
		dy := position.Y - tg.lastPosition.Y
		dz := position.Z - tg.lastPosition.Z
		distance := math.Sqrt(dx*dx + dy*dy + dz*dz)
		data.Speed = distance / dt
	}

	// Add noise to speed
	data.Speed = tg.noise.GaussianNoise(data.Speed, 3.0)
	if data.Speed < 0 {
		data.Speed = 0
	}

	// Generate joint angles (simplified inverse kinematics approximation)
	tg.generateJointAngles(data, position)

	// Generate grip force
	tg.generateGripForce(data, gripper, gripperPosition, phaseProgress)

	// Add position noise
	data.PositionX = tg.noise.GaussianNoise(position.X, 0.1) // ±0.1mm accuracy
	data.PositionY = tg.noise.GaussianNoise(position.Y, 0.1)
	data.PositionZ = tg.noise.GaussianNoise(position.Z, 0.1)

	// Update last position for speed calculation
	tg.lastPosition = position
	tg.lastTime = now

	return data
}

func (tg *TimeseriesGenerator) generateJointAngles(data *PickerData, position Position3D) {
	// Simplified joint angle calculation
	// In reality, this would use proper inverse kinematics

	// J1 (base rotation): angle to X-Y position
	data.Joint1 = math.Atan2(position.Y, position.X) * 180 / math.Pi

	// Distance from base in X-Y plane
	xyDistance := math.Sqrt(position.X*position.X + position.Y*position.Y)

	// J2 (shoulder): rough approximation based on reach
	reachRatio := xyDistance / (tg.config.MaxReachX * 0.7) // Assume arm length is 70% of max reach
	if reachRatio > 1 {
		reachRatio = 1
	}
	data.Joint2 = 45 + reachRatio*45 // 45-90 degrees

	// J3 (elbow): complementary to J2
	data.Joint3 = 180 - data.Joint2*1.5

	// J4, J5, J6 (wrist): keep tool pointing down
	data.Joint4 = 0
	data.Joint5 = 90 - (data.Joint2 + data.Joint3 - 90) // Keep tool vertical
	data.Joint6 = -data.Joint1 // Counter-rotate to maintain tool orientation

	// Add small noise to joint angles
	data.Joint1 = tg.noise.GaussianNoise(data.Joint1, 0.1)
	data.Joint2 = tg.noise.GaussianNoise(data.Joint2, 0.1)
	data.Joint3 = tg.noise.GaussianNoise(data.Joint3, 0.1)
	data.Joint4 = tg.noise.GaussianNoise(data.Joint4, 0.1)
	data.Joint5 = tg.noise.GaussianNoise(data.Joint5, 0.1)
	data.Joint6 = tg.noise.GaussianNoise(data.Joint6, 0.1)
}

func (tg *TimeseriesGenerator) generateGripForce(data *PickerData, gripper GripperState, gripperPosition float64, phaseProgress float64) {
	switch gripper {
	case GripperOpen:
		data.GripForce = 0

	case GripperClosing:
		// Force builds as gripper closes
		// Higher force near the end as it contacts the part
		if gripperPosition > 80 {
			// Contacting part - force increases rapidly
			contactProgress := (gripperPosition - 80) / 20
			data.GripForce = tg.config.MaxGripForce * 0.3 * contactProgress
		} else {
			// Just air resistance
			data.GripForce = tg.noise.GaussianNoise(2.0, 0.5)
		}

	case GripperClosed:
		// Full grip force on part
		data.GripForce = tg.noise.ColoredNoise("gripForce", tg.config.MaxGripForce*0.7, 5.0, 0.6)

	case GripperOpening:
		// Force decreases as gripper opens
		if gripperPosition > 80 {
			releaseProgress := (100 - gripperPosition) / 20
			data.GripForce = tg.config.MaxGripForce * 0.7 * (1 - releaseProgress)
		} else {
			data.GripForce = tg.noise.GaussianNoise(1.0, 0.3)
		}
	}

	// Ensure non-negative
	if data.GripForce < 0 {
		data.GripForce = 0
	}
}
