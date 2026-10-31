package utils

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestDateIn_EachZoneItsOwnDate - the same instant is two calendar dates in two zones: 01:30 UTC is
// still yesterday in La Paz.
func TestDateIn_EachZoneItsOwnDate(t *testing.T) {
	instant := time.Date(2026, 3, 10, 1, 30, 0, 0, time.UTC)

	assert.Equal(t, time.Date(2026, 3, 9, 0, 0, 0, 0, time.UTC), DateIn(instant, "America/La_Paz"))
	assert.Equal(t, time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC), DateIn(instant, "Europe/Madrid"))
}

// TestDateIn_UnknownZoneIsTheDefault - a stored value the runtime cannot load never fails a request.
func TestDateIn_UnknownZoneIsTheDefault(t *testing.T) {
	instant := time.Date(2026, 3, 10, 1, 30, 0, 0, time.UTC)

	assert.Equal(t, time.Date(2026, 3, 9, 0, 0, 0, 0, time.UTC), DateIn(instant, "Nowhere/Land"))
	assert.Equal(t, time.Date(2026, 3, 9, 0, 0, 0, 0, time.UTC), DateIn(instant, ""))
}
