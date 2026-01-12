package erp

import (
	"fmt"
	"time"

	"github.com/sebastiankruger/shopfloor-simulator/internal/config"
	"github.com/sebastiankruger/shopfloor-simulator/internal/simulator"
)

// ShiftSchedule defines the shift schedule
type ShiftSchedule struct {
	Name   string
	Start  int // Hour (0-23)
	End    int // Hour (0-23)
	Breaks []BreakDefinition
}

// BreakDefinition defines a break within a shift
type BreakDefinition struct {
	StartHour   int
	StartMinute int
	EndHour     int
	EndMinute   int
	Type        string
}

// ShiftManager manages shift schedules and transitions
type ShiftManager struct {
	cfg          *config.Config
	location     *time.Location
	schedules    []ShiftSchedule
	currentShift *simulator.Shift
	workCenterID string
}

// NewShiftManager creates a new shift manager
func NewShiftManager(cfg *config.Config) (*ShiftManager, error) {
	loc, err := time.LoadLocation(cfg.Timezone)
	if err != nil {
		loc = time.Local
	}

	sm := &ShiftManager{
		cfg:          cfg,
		location:     loc,
		workCenterID: "WC-WELD-01",
	}

	// Initialize schedules based on shift model
	sm.initializeSchedules()

	return sm, nil
}

func (sm *ShiftManager) initializeSchedules() {
	switch sm.cfg.ShiftModel {
	case "3-shift":
		sm.schedules = []ShiftSchedule{
			{
				Name:  "Morning",
				Start: 6,
				End:   14,
				Breaks: []BreakDefinition{
					{StartHour: 9, StartMinute: 0, EndHour: 9, EndMinute: 1, Type: "break"},
					{StartHour: 12, StartMinute: 0, EndHour: 12, EndMinute: 1, Type: "lunch"},
				},
			},
			{
				Name:  "Afternoon",
				Start: 14,
				End:   22,
				Breaks: []BreakDefinition{
					{StartHour: 17, StartMinute: 0, EndHour: 17, EndMinute: 1, Type: "break"},
					{StartHour: 19, StartMinute: 0, EndHour: 19, EndMinute: 1, Type: "lunch"},
				},
			},
			{
				Name:  "Night",
				Start: 22,
				End:   6, // Next day
				Breaks: []BreakDefinition{
					{StartHour: 1, StartMinute: 0, EndHour: 1, EndMinute: 1, Type: "break"},
					{StartHour: 3, StartMinute: 0, EndHour: 3, EndMinute: 1, Type: "lunch"},
				},
			},
		}

	case "2-shift":
		sm.schedules = []ShiftSchedule{
			{
				Name:  "Day",
				Start: 6,
				End:   14,
				Breaks: []BreakDefinition{
					{StartHour: 9, StartMinute: 0, EndHour: 9, EndMinute: 1, Type: "break"},
					{StartHour: 12, StartMinute: 0, EndHour: 12, EndMinute: 1, Type: "lunch"},
				},
			},
			{
				Name:  "Late",
				Start: 14,
				End:   22,
				Breaks: []BreakDefinition{
					{StartHour: 17, StartMinute: 0, EndHour: 17, EndMinute: 1, Type: "break"},
					{StartHour: 19, StartMinute: 0, EndHour: 19, EndMinute: 1, Type: "lunch"},
				},
			},
		}

	default: // 1-shift
		sm.schedules = []ShiftSchedule{
			{
				Name:  "Day",
				Start: 8,
				End:   17,
				Breaks: []BreakDefinition{
					{StartHour: 10, StartMinute: 0, EndHour: 10, EndMinute: 1, Type: "break"},
					{StartHour: 12, StartMinute: 30, EndHour: 12, EndMinute: 31, Type: "lunch"},
					{StartHour: 15, StartMinute: 0, EndHour: 15, EndMinute: 1, Type: "break"},
				},
			},
		}
	}
}

// GetCurrentShift returns the current shift based on the current time
func (sm *ShiftManager) GetCurrentShift(now time.Time) *simulator.Shift {
	localNow := now.In(sm.location)
	hour := localNow.Hour()

	for i, sched := range sm.schedules {
		var inShift bool

		if sched.End > sched.Start {
			// Normal shift (doesn't cross midnight)
			inShift = hour >= sched.Start && hour < sched.End
		} else {
			// Night shift (crosses midnight)
			inShift = hour >= sched.Start || hour < sched.End
		}

		if inShift {
			return sm.createShift(localNow, sched, i+1)
		}
	}

	// Default to first shift if no match (shouldn't happen with 24/7 schedule)
	return sm.createShift(localNow, sm.schedules[0], 1)
}

func (sm *ShiftManager) createShift(now time.Time, sched ShiftSchedule, shiftNumber int) *simulator.Shift {
	// Calculate shift start and end times for today
	startTime := time.Date(now.Year(), now.Month(), now.Day(), sched.Start, 0, 0, 0, sm.location)
	endTime := time.Date(now.Year(), now.Month(), now.Day(), sched.End, 0, 0, 0, sm.location)

	// Adjust for night shift
	if sched.End < sched.Start {
		if now.Hour() < sched.End {
			// We're in the morning part of night shift
			startTime = startTime.AddDate(0, 0, -1)
		} else {
			// We're in the evening part of night shift
			endTime = endTime.AddDate(0, 0, 1)
		}
	}

	// Create planned breaks
	breaks := make([]simulator.PlannedBreak, 0, len(sched.Breaks))
	for _, b := range sched.Breaks {
		breakStart := time.Date(now.Year(), now.Month(), now.Day(), b.StartHour, b.StartMinute, 0, 0, sm.location)
		breakEnd := time.Date(now.Year(), now.Month(), now.Day(), b.EndHour, b.EndMinute, 0, 0, sm.location)

		// Adjust for night shift breaks that are in the next day
		if sched.End < sched.Start && b.StartHour < sched.End {
			if now.Hour() >= sched.Start {
				breakStart = breakStart.AddDate(0, 0, 1)
				breakEnd = breakEnd.AddDate(0, 0, 1)
			}
		}

		breaks = append(breaks, simulator.PlannedBreak{
			Start: breakStart,
			End:   breakEnd,
			Type:  b.Type,
		})
	}

	shiftID := fmt.Sprintf("SHIFT-%s-%s", startTime.Format("2006-01-02"), sched.Name[:1])

	return &simulator.Shift{
		ShiftID:       shiftID,
		ShiftName:     sched.Name,
		ShiftNumber:   shiftNumber,
		StartTime:     startTime,
		EndTime:       endTime,
		WorkCenterID:  sm.workCenterID,
		PlannedBreaks: breaks,
		Status:        simulator.ShiftStatusActive,
	}
}

// IsBreakTime checks if the current time is during a break
func (sm *ShiftManager) IsBreakTime(now time.Time, shift *simulator.Shift) bool {
	if shift == nil {
		return false
	}

	localNow := now.In(sm.location)

	for _, b := range shift.PlannedBreaks {
		if localNow.After(b.Start) && localNow.Before(b.End) {
			return true
		}
	}

	return false
}

// HasShiftChanged checks if the shift has changed and returns the new shift if so
func (sm *ShiftManager) HasShiftChanged(now time.Time) (*simulator.Shift, bool) {
	newShift := sm.GetCurrentShift(now)

	if sm.currentShift == nil {
		sm.currentShift = newShift
		return newShift, true
	}

	if sm.currentShift.ShiftID != newShift.ShiftID {
		oldShift := sm.currentShift
		oldShift.Status = simulator.ShiftStatusEnded
		sm.currentShift = newShift
		return newShift, true
	}

	return nil, false
}

// GetCurrentShiftRef returns a reference to the current shift
func (sm *ShiftManager) GetCurrentShiftRef() *simulator.Shift {
	return sm.currentShift
}
