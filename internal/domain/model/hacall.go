package model

import "fmt"

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
