package services

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// StreamLag is how old the oldest point a stream's consumer group has not finished is: the first entry
// not yet delivered to the group, or the oldest one delivered and not acked; 0 when it is drained or
// has no group. Three reads (XINFO GROUPS, XRANGE, XPENDING) — the GPS consumer's gauge and the load
// test's verdict (INFRA-018, TRACK-010).
func StreamLag(ctx context.Context, client *redis.Client, stream string) float64 {
	groups, err := client.XInfoGroups(ctx, stream).Result()
	if err != nil || len(groups) == 0 {
		return 0
	}
	next, pendingLower := "", ""
	if entries, err := client.XRangeN(ctx, stream, "("+groups[0].LastDeliveredID, "+", 1).Result(); err == nil && len(entries) > 0 {
		next = entries[0].ID
	}
	if pending, err := client.XPending(ctx, stream, groups[0].Name).Result(); err == nil && pending.Count > 0 {
		pendingLower = pending.Lower
	}
	return lagSeconds(time.Now(), next, pendingLower)
}

// lagSeconds is the age at now of the older of two entry ids, either of which may be "" (none).
// An entry id's first half is its milliseconds.
func lagSeconds(now time.Time, next, pendingLower string) float64 {
	oldest := entryMillis(next)
	if ms := entryMillis(pendingLower); ms != 0 && (oldest == 0 || ms < oldest) {
		oldest = ms
	}
	if oldest == 0 {
		return 0
	}
	return float64(now.UnixMilli()-oldest) / 1000
}

func entryMillis(id string) int64 {
	ms, _ := strconv.ParseInt(strings.SplitN(id, "-", 2)[0], 10, 64)
	return ms
}
