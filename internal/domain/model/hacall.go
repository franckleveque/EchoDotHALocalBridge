package model

import (
	"errors"
	"fmt"
)

var (
	// ErrTooManyRequests is returned when the concurrent HA call limit is reached.
	ErrTooManyRequests = errors.New("too many concurrent requests, please try again later")
	// ErrInvalidTestRequest is returned when a test request is missing its device or state.
	ErrInvalidTestRequest = errors.New("invalid test request")
)

// HAAPIError is returned when the Home Assistant API answers with a non-success status.
type HAAPIError struct {
	Status int
	Body   string
}

func (e *HAAPIError) Error() string {
	if e.Body == "" {
		return fmt.Sprintf("HA API error: %d", e.Status)
	}
	return fmt.Sprintf("HA API error: %d: %s", e.Status, e.Body)
}
