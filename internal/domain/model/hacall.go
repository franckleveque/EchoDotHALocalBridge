package model

import (
	"errors"
	"fmt"
	"slices"
	"strings"
)

var (
	// ErrTooManyRequests is returned when the concurrent HA call limit is reached.
	ErrTooManyRequests = errors.New("too many concurrent requests, please try again later")
	// ErrInvalidTestRequest is returned when a test request is missing its device or state.
	ErrInvalidTestRequest = errors.New("invalid test request")
	// ErrNoOpAction is returned when the device is configured as No-Op for the requested action.
	ErrNoOpAction = errors.New("no-op configured for this action: nothing sent to Home Assistant")
)

// HAAPIError is returned when the Home Assistant API answers with a non-success status.
type HAAPIError struct {
	Status int
	Body   string
}

// Error joins the non-empty segments (status, then body) with ": ".
func (e *HAAPIError) Error() string {
	parts := []string{fmt.Sprintf("HA API error: %d", e.Status), e.Body}
	return strings.Join(slices.DeleteFunc(parts, func(p string) bool { return p == "" }), ": ")
}
