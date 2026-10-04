package routing

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"josex/web/modules/geo/interfaces"
	"josex/web/modules/geo/models"
)

var oneRider = &models.OptimizeProblem{
	Costing:  "bus",
	Vehicles: []models.OptimizeVehicle{{ID: 1, End: cochabamba[0], Capacity: 10, TimeWindow: [2]int{18000, 27900}}},
	Jobs:     []models.OptimizeJob{{ID: 1, Location: cochabamba[1], ServiceS: 60}},
}

// TestVroomSolution - the job steps become the route's stops in order, the end step its arrival, and
// the request carries [lng, lat] points, the seats and the geometry flag.
func TestVroomSolution(t *testing.T) {
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		for _, want := range []string{`"end":[-66.157,-17.3935]`, `"capacity":[10]`, `"g":true`, `"pickup":[1]`} {
			if !strings.Contains(string(body), want) {
				t.Errorf("request lacks %s: %s", want, body)
			}
		}
		_, _ = w.Write([]byte(`{"code":0,"unassigned":[{"id":7}],"routes":[{"vehicle":1,"distance":4200,"steps":[
			{"type":"start","arrival":18000},{"type":"job","job":1,"arrival":18000},{"type":"end","arrival":18600}]}]}`))
	}))
	defer stub.Close()

	solution, err := newVroomProvider(stub.URL, time.Second).Optimize(context.Background(), oneRider)
	if err != nil {
		t.Fatalf("optimize: %v", err)
	}
	if len(solution.Routes) != 1 || len(solution.Routes[0].Stops) != 1 {
		t.Fatalf("want one route with one stop, got %+v", solution.Routes)
	}
	route := solution.Routes[0]
	if route.EndS != 18600 || route.DurationS != 600 || route.DistanceM != 4200 {
		t.Errorf("route = %+v", route)
	}
	if len(solution.Unassigned) != 1 || solution.Unassigned[0] != 7 {
		t.Errorf("unassigned = %v", solution.Unassigned)
	}
}

// TestVroomDownIsUnavailable - a refused connection and a 500 both answer ErrOptimizerUnavailable.
func TestVroomDownIsUnavailable(t *testing.T) {
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer stub.Close()
	for _, url := range []string{stub.URL, "http://127.0.0.1:1"} {
		_, err := newVroomProvider(url, time.Second).Optimize(context.Background(), oneRider)
		if !errors.Is(err, interfaces.ErrOptimizerUnavailable) {
			t.Errorf("%s: want ErrOptimizerUnavailable, got %v", url, err)
		}
	}
}
