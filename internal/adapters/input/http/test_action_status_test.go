package http

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"hue-bridge-emulator/internal/domain/model"
)

func TestStatusForTestError(t *testing.T) {
	cases := []struct {
		err  error
		want int
	}{
		{model.ErrInvalidTestRequest, http.StatusBadRequest},
		{model.ErrNoOpAction, http.StatusConflict},
		{model.ErrTooManyRequests, http.StatusTooManyRequests},
		{fmt.Errorf("wrapped: %w", model.ErrNoOpAction), http.StatusConflict},
		{&model.HAAPIError{Status: 500}, http.StatusBadGateway},
		{context.DeadlineExceeded, http.StatusBadGateway},
	}
	for _, c := range cases {
		if got := statusForTestError(c.err); got != c.want {
			t.Errorf("%v: got %d want %d", c.err, got, c.want)
		}
	}
}
