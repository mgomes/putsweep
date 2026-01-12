package scheduler

import (
	"time"

	"github.com/mgomes/putsweep/internal/config"
)

type Scheduler struct {
	mode config.ScheduleMode
}

func New(mode config.ScheduleMode) *Scheduler {
	return &Scheduler{mode: mode}
}

func (s *Scheduler) SetMode(mode config.ScheduleMode) {
	s.mode = mode
}

func (s *Scheduler) Mode() config.ScheduleMode {
	return s.mode
}

func (s *Scheduler) CanStartNew() bool {
	now := time.Now()

	switch s.mode {
	case config.ScheduleAnyTime:
		return true

	case config.ScheduleAfterMidnight:
		hour := now.Hour()
		return hour >= 0 && hour < 6

	case config.ScheduleBusinessHours:
		weekday := now.Weekday()
		if weekday == time.Saturday || weekday == time.Sunday {
			return false
		}
		hour := now.Hour()
		return hour >= 9 && hour < 17
	}

	return true
}

func (s *Scheduler) NextActiveTime() *time.Time {
	now := time.Now()

	switch s.mode {
	case config.ScheduleAnyTime:
		return nil

	case config.ScheduleAfterMidnight:
		hour := now.Hour()
		if hour >= 0 && hour < 6 {
			return nil
		}
		next := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
		return &next

	case config.ScheduleBusinessHours:
		weekday := now.Weekday()
		hour := now.Hour()

		if weekday != time.Saturday && weekday != time.Sunday {
			if hour < 9 {
				next := time.Date(now.Year(), now.Month(), now.Day(), 9, 0, 0, 0, now.Location())
				return &next
			}
			if hour >= 9 && hour < 17 {
				return nil
			}
		}

		next := now
		for {
			next = next.AddDate(0, 0, 1)
			if next.Weekday() != time.Saturday && next.Weekday() != time.Sunday {
				result := time.Date(next.Year(), next.Month(), next.Day(), 9, 0, 0, 0, next.Location())
				return &result
			}
		}
	}

	return nil
}

func (s *Scheduler) StatusText() string {
	if s.CanStartNew() {
		switch s.mode {
		case config.ScheduleAnyTime:
			return "Active"
		case config.ScheduleAfterMidnight:
			return "Active (until 6 AM)"
		case config.ScheduleBusinessHours:
			return "Active (until 5 PM)"
		}
	}

	next := s.NextActiveTime()
	if next == nil {
		return "Waiting"
	}

	return "Next: " + next.Format("Mon 3:04 PM")
}
