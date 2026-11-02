package http

import (
	"errors"
	"net/http"

	"hue-bridge-emulator/internal/domain/model"
)

// testActionStatuses maps domain errors to the HTTP status of the test-action route.
var testActionStatuses = []struct {
	err    error
	status int
}{
	{model.ErrInvalidTestRequest, http.StatusBadRequest},
	{model.ErrNoOpAction, http.StatusConflict},
	{model.ErrTooManyRequests, http.StatusTooManyRequests},
}

// statusForTestError returns the HTTP status for a TestDeviceAction error.
// Anything else (HA API errors, network errors, timeouts) is an upstream failure.
func statusForTestError(err error) int {
	for _, entry := range testActionStatuses {
		if errors.Is(err, entry.err) {
			return entry.status
		}
	}
	return http.StatusBadGateway
}
