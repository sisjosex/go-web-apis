package services

import (
	"testing"
	"time"
)

func TestLagSeconds(t *testing.T) {
	now := time.UnixMilli(1_700_000_010_000)
	cases := []struct {
		name, next, pending string
		want                float64
	}{
		{"drained", "", "", 0},
		{"undelivered only", "1700000007500-0", "", 2.5},
		{"pending only", "", "1700000004000-3", 6},
		{"pending older than next", "1700000009000-0", "1700000004000-0", 6},
		{"next older than pending", "1700000001000-0", "1700000004000-0", 9},
		{"unparseable id is none", "junk", "", 0},
	}
	for _, c := range cases {
		if got := lagSeconds(now, c.next, c.pending); got != c.want {
			t.Errorf("%s: lagSeconds = %v, want %v", c.name, got, c.want)
		}
	}
}
