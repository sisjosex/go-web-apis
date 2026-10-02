// Command gpsload drives the GPS ingest the way a fleet of devices would (TRACK-010 step 5): each
// simulated vehicle posts one point every -interval with its own device token, and the run reports
// the ingest latency, how far the worker fell behind the stream, and whether every point was stored
// exactly once.
//
//	go run ./tools/gpsload -tenant test-company -vehicles 200 -interval 5s -duration 2m
//
// It reads the platform database and Valkey from ENV_FILE (default .env.platform; exported variables
// win), creates LOAD-0001… vehicles in the tenant's first carrier when they are missing and issues each
// a device token. The API it posts to must run a worker too (-role=all or a separate worker).
package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"josex/web/config"
	coreServices "josex/web/modules/core/services"
	"josex/web/modules/core/utils"
	tenancyRepos "josex/web/modules/tenancy/repositories"
	"josex/web/modules/tracking/models"
	trackingRepos "josex/web/modules/tracking/repositories"
	trackingServices "josex/web/modules/tracking/services"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// Where the simulated fleet drives: small circles around central Cochabamba (INFRA-003 D1).
const baseLat, baseLng = -17.3935, -66.1570

type vehicle struct {
	id    uuid.UUID
	token string
}

func main() {
	apiURL := flag.String("url", "http://127.0.0.1:8080/api/v1", "API base URL")
	slug := flag.String("tenant", "", "Tenant slug the load vehicles belong to (required)")
	count := flag.Int("vehicles", 200, "Simulated vehicles")
	interval := flag.Duration("interval", 5*time.Second, "Time between two points of one vehicle")
	duration := flag.Duration("duration", 2*time.Minute, "How long to post")
	flag.Parse()
	if *slug == "" {
		flag.Usage()
		os.Exit(2)
	}
	if os.Getenv("ENV_FILE") == "" {
		_ = os.Setenv("ENV_FILE", ".env.platform")
	}
	utils.LoadEnv()
	config.GetConfig()

	ctx := context.Background()
	db := coreServices.NewDatabaseService()
	db.InitDatabase(ctx)
	defer db.CloseDatabase(ctx)
	valkey, err := coreServices.NewValkeyService(config.ModularAppConfig.Core.RedisURL)
	if err != nil || valkey == nil {
		log.Fatalf("REDIS_URL is required: the lag is read from the stream (%v)", err)
	}

	tenantID, tenantCtx, fleet := setup(ctx, db, *slug, *count)
	stream := trackingServices.GPSStreamIn(tenantCtx, tenantID.String())
	log.Printf("🚌 %d vehicles, one point every %s for %s → %s", len(fleet), *interval, *duration, *apiURL)

	start := time.Now()
	stop, lagDone := make(chan struct{}), make(chan float64, 1)
	go func() { lagDone <- watchLag(ctx, valkey.Client(), stream, stop) }()
	latencies, sent, failed := post(fleet, *apiURL, *interval, *duration)
	close(stop)
	maxLag := <-lagDone

	// Let the worker finish what is queued, then count what it stored.
	drainUntil := time.Now().Add(15 * time.Second)
	for time.Now().Before(drainUntil) && streamLag(ctx, valkey.Client(), stream) > 0 {
		time.Sleep(200 * time.Millisecond)
	}
	stored, distinct := countStored(tenantCtx, db, fleet, start.Add(-time.Minute))

	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
	fmt.Printf("\nrequests  %d sent, %d failed\n", sent, failed)
	fmt.Printf("latency   p50 %s · p95 %s · p99 %s · max %s\n",
		pct(latencies, 50), pct(latencies, 95), pct(latencies, 99), pct(latencies, 100))
	fmt.Printf("lag       max %.2fs behind the stream\n", maxLag)
	fmt.Printf("stored    %d rows, %d distinct (vehicle, recorded_at), %d points sent\n", stored, distinct, sent-failed)

	ok := failed == 0 && pct(latencies, 95) < 100*time.Millisecond && maxLag < 5 && stored == distinct && stored == int64(sent-failed)
	if !ok {
		fmt.Println("RESULT    FAIL (p95 < 100ms, lag < 5s, every point stored once)")
		os.Exit(1)
	}
	fmt.Println("RESULT    PASS")
}

// setup answers the tenant, a context on its database, and count vehicles with fresh tokens.
func setup(ctx context.Context, db coreServices.DatabaseService, slug string, count int) (uuid.UUID, context.Context, []vehicle) {
	tenant, err := tenancyRepos.NewTenantRepository(db).GetTenantBySlug(ctx, slug)
	if err != nil {
		log.Fatalf("tenant %s: %v", slug, err)
	}
	tenantCtx := ctx
	if tenant.DatabaseURL != nil && *tenant.DatabaseURL != "" {
		tenantCtx = context.WithValue(ctx, coreServices.TenantDatabaseURLKey, *tenant.DatabaseURL)
	}
	repo := trackingRepos.NewTrackingRepository(db)
	companies, _, err := repo.ListCompanies(tenantCtx, tenant.ID, models.ListCompaniesQuery{Page: 1, PageSize: 1})
	if err != nil || len(companies) == 0 {
		log.Fatalf("tenant %s needs a transport company (%v)", slug, err)
	}

	existing := map[string]uuid.UUID{}
	for page := 1; ; page++ {
		rows, total, err := repo.ListVehicles(tenantCtx, tenant.ID, models.ListVehiclesQuery{Search: "LOAD-", Page: page, PageSize: 100})
		if err != nil {
			log.Fatalf("list vehicles: %v", err)
		}
		for _, v := range rows {
			existing[v.PlateNumber] = v.ID
		}
		if int64(page*100) >= total {
			break
		}
	}

	devices := tenancyRepos.NewGPSDeviceRepository(db)
	fleet := make([]vehicle, 0, count)
	for i := 1; i <= count; i++ {
		plate := fmt.Sprintf("LOAD-%04d", i)
		id, ok := existing[plate]
		if !ok {
			v, err := repo.CreateVehicle(tenantCtx, tenant.ID, &models.CreateVehicleDto{
				CompanyID: companies[0].ID, PlateNumber: plate, VehicleType: "bus", Capacity: 40, Status: "active",
			})
			if err != nil {
				log.Fatalf("create %s: %v", plate, err)
			}
			id = v.ID
		}
		secret := make([]byte, 32)
		_, _ = rand.Read(secret)
		token := tenancyRepos.GPSDeviceToken(tenant.ID, secret)
		_, hash, _ := tenancyRepos.ParseGPSDeviceToken(token)
		if err := devices.Issue(ctx, tenant.ID, id, hash); err != nil {
			log.Fatalf("issue token for %s: %v", plate, err)
		}
		fleet = append(fleet, vehicle{id: id, token: token})
	}
	return tenant.ID, tenantCtx, fleet
}

// post runs every vehicle for duration, each starting at its own offset inside the first interval so
// the load is even, and answers every request's latency.
func post(fleet []vehicle, apiURL string, interval, duration time.Duration) ([]time.Duration, int, int) {
	client := &http.Client{Timeout: 5 * time.Second, Transport: &http.Transport{MaxIdleConnsPerHost: len(fleet)}}

	var mu sync.Mutex
	var latencies []time.Duration
	sent, failed := 0, 0
	var wg sync.WaitGroup
	deadline := time.Now().Add(duration)
	for i, v := range fleet {
		wg.Add(1)
		go func(i int, v vehicle) {
			defer wg.Done()
			time.Sleep(time.Duration(i) * interval / time.Duration(len(fleet)))
			for n := 0; time.Now().Before(deadline); n++ {
				angle := float64(n)/20 + float64(i)
				body, _ := json.Marshal([]map[string]any{{
					"recorded_at": time.Now().UTC().Format(time.RFC3339Nano),
					"lat":         baseLat + 0.01*math.Sin(angle) + float64(i%20)*0.001,
					"lng":         baseLng + 0.01*math.Cos(angle) + float64(i/20)*0.001,
					"speed":       8.3,
					"heading":     math.Mod(angle*57.3, 360),
				}})
				req, _ := http.NewRequest(http.MethodPost, apiURL+"/tracking/ingest/positions", bytes.NewReader(body))
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("X-Device-Token", v.token)
				began := time.Now()
				res, err := client.Do(req)
				took := time.Since(began)
				ok := err == nil && res.StatusCode == http.StatusAccepted
				if res != nil {
					_ = res.Body.Close()
				}
				mu.Lock()
				sent++
				if ok {
					latencies = append(latencies, took)
				} else {
					failed++
					if failed <= 3 {
						log.Printf("⚠️  post failed: %v %s", err, statusOf(res))
					}
				}
				mu.Unlock()
				time.Sleep(interval - took)
			}
		}(i, v)
	}
	wg.Wait()
	return latencies, sent, failed
}

func statusOf(res *http.Response) string {
	if res == nil {
		return ""
	}
	return res.Status
}

// watchLag samples the stream every 500 ms until stop and answers the largest lag seen, in seconds.
func watchLag(ctx context.Context, client *redis.Client, stream string, stop <-chan struct{}) float64 {
	worst := 0.0
	for {
		select {
		case <-stop:
			return worst
		case <-time.After(500 * time.Millisecond):
			worst = math.Max(worst, streamLag(ctx, client, stream))
		}
	}
}

// streamLag is how old the oldest point the worker has not finished is: the first entry not yet
// delivered to the group, or the oldest one delivered and not acked. Entry ids are milliseconds.
func streamLag(ctx context.Context, client *redis.Client, stream string) float64 {
	oldest := int64(0)
	groups, err := client.XInfoGroups(ctx, stream).Result()
	if err != nil || len(groups) == 0 {
		return 0
	}
	if next, err := client.XRangeN(ctx, stream, "("+groups[0].LastDeliveredID, "+", 1).Result(); err == nil && len(next) > 0 {
		oldest = entryMillis(next[0].ID)
	}
	if pending, err := client.XPending(ctx, stream, groups[0].Name).Result(); err == nil && pending.Count > 0 {
		if ms := entryMillis(pending.Lower); oldest == 0 || ms < oldest {
			oldest = ms
		}
	}
	if oldest == 0 {
		return 0
	}
	return float64(time.Now().UnixMilli()-oldest) / 1000
}

func entryMillis(id string) int64 {
	ms, _ := strconv.ParseInt(strings.SplitN(id, "-", 2)[0], 10, 64)
	return ms
}

// countStored answers how many rows the load vehicles have since the run began, and how many distinct
// (vehicle, recorded_at) pairs they are.
func countStored(ctx context.Context, db coreServices.DatabaseService, fleet []vehicle, since time.Time) (int64, int64) {
	ids := make([]uuid.UUID, len(fleet))
	for i, v := range fleet {
		ids[i] = v.id
	}
	var stored, distinct int64
	if err := db.QueryRow(ctx, `SELECT count(*), count(DISTINCT (vehicle_id, recorded_at))
		FROM tracking.vehicle_positions WHERE vehicle_id = ANY($1) AND recorded_at >= $2`, ids, since).Scan(&stored, &distinct); err != nil {
		log.Fatalf("count stored points: %v", err)
	}
	return stored, distinct
}

func pct(sorted []time.Duration, p int) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	i := int(math.Ceil(float64(p)/100*float64(len(sorted)))) - 1
	if i < 0 {
		i = 0
	}
	return sorted[i]
}
