package model

import "testing"

func TestHAAPIError_Error(t *testing.T) {
	e := &HAAPIError{Status: 400, Body: `{"message":"x"}`}
	if got, want := e.Error(), `HA API error: 400: {"message":"x"}`; got != want {
		t.Errorf("got %q want %q", got, want)
	}
	e = &HAAPIError{Status: 400}
	if got, want := e.Error(), "HA API error: 400"; got != want {
		t.Errorf("got %q want %q", got, want)
	}
}
