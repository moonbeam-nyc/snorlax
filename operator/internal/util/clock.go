package util

import "time"

// Clock abstracts time based operations necessary for easier testing
type Clock interface {
	Now(location *time.Location) time.Time
}

// RealTime provides actual system time
type RealTime struct{}

func (RealTime) Now(location *time.Location) time.Time {
	return time.Now().In(location)
}

// MockTime provides a mock time
type MockTime struct {
	Time time.Time
}

func (m MockTime) Now(location *time.Location) time.Time {
	return m.Time.In(location)
}
