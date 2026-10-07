package utils

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// A bare number reads in the unit its key names, a duration string as written (MOBILE-022: "168" under
// JWT_REFRESH_EXPIRATION_HOURS used to mean 168 minutes).
func TestGetEnvAsDurationUnit(t *testing.T) {
	cases := []struct {
		name  string
		key   string
		value string
		unit  time.Duration
		want  time.Duration
	}{
		{"bare number in hours", "MOBILE022_REFRESH_HOURS", "168", time.Hour, 168 * time.Hour},
		{"duration string", "MOBILE022_REFRESH_HOURS", "720h", time.Hour, 720 * time.Hour},
		{"bare number in minutes", "MOBILE022_ACCESS_MINUTES", "15", time.Minute, 15 * time.Minute},
		{"invalid falls back to default", "MOBILE022_BAD_HOURS", "soon", time.Hour, 7 * time.Hour},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(tc.key, tc.value)

			got := GetEnvAsDurationUnit(tc.key, 7*time.Hour, tc.unit)

			assert.Equal(t, tc.want, got)
		})
	}
}

// The unset key keeps the default; GetEnvAsDuration still reads a bare number as minutes.
func TestGetEnvAsDuration_DefaultsAndMinutes(t *testing.T) {
	t.Setenv("MOBILE022_POLL", "5")

	assert.Equal(t, 5*time.Minute, GetEnvAsDuration("MOBILE022_POLL", time.Second))
	assert.Equal(t, 3*time.Hour, GetEnvAsDurationUnit("MOBILE022_UNSET_HOURS", 3*time.Hour, time.Hour))
}
