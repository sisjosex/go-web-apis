package models

// Place is one address-search result: a street, a named place or a POI, as a point.
type Place struct {
	Name string  `json:"name"`
	Kind string  `json:"kind"` // street | place | poi
	City *string `json:"city"`
	Lat  float64 `json:"lat"`
	Lng  float64 `json:"lng"`
}

// GeocodeQuery is GET /geo/geocode. lat/lng, when sent, rank nearby matches first.
type GeocodeQuery struct {
	Q     string   `form:"q" binding:"required,min=2,max=100"`
	Lat   *float64 `form:"lat" binding:"omitempty,latitude"`
	Lng   *float64 `form:"lng" binding:"omitempty,longitude"`
	Limit int      `form:"limit" binding:"omitempty,min=1,max=20"`
}

// GeocodeResponse wraps the results so the shape can grow without breaking clients.
type GeocodeResponse struct {
	Results []Place `json:"results"`
}

// ReverseQuery is GET /geo/reverse.
type ReverseQuery struct {
	Lat *float64 `form:"lat" binding:"required,latitude"`
	Lng *float64 `form:"lng" binding:"required,longitude"`
}

// LatLng is a WGS84 point in a request body.
type LatLng struct {
	Lat float64 `json:"lat" binding:"latitude"`
	Lng float64 `json:"lng" binding:"longitude"`
}

// RouteRequest is POST /geo/route: the locations in visiting order.
type RouteRequest struct {
	Locations []LatLng `json:"locations" binding:"required,min=2,max=20,dive"`
	Costing   string   `json:"costing" binding:"required,oneof=bus pedestrian"`
}

// RouteLeg is the stretch between two consecutive locations.
type RouteLeg struct {
	Polyline6 string  `json:"polyline6"`
	DurationS float64 `json:"duration_s"`
	DistanceM float64 `json:"distance_m"`
}

// Route is the whole trip: polyline6 is every leg's shape joined, precision 6 as Valhalla encodes it.
type Route struct {
	Polyline6 string     `json:"polyline6"`
	DurationS float64    `json:"duration_s"`
	DistanceM float64    `json:"distance_m"`
	Legs      []RouteLeg `json:"legs"`
}

// Trace is a recorded path matched to the roads: the line, precision 6, and its length.
type Trace struct {
	Polyline6 string  `json:"polyline6"`
	DistanceM float64 `json:"distance_m"`
}

// EtaRequest is POST /geo/eta.
type EtaRequest struct {
	From    *LatLng `json:"from" binding:"required"`
	To      *LatLng `json:"to" binding:"required"`
	Costing string  `json:"costing" binding:"required,oneof=bus pedestrian"`
}

// Where an ETA came from, best first (D5).
const (
	EstimateValhalla   = "valhalla"
	EstimateHistorical = "historical"
	EstimateFallback   = "fallback"
)

// Eta always answers: a degraded estimate is flagged by EstimateSource, never an error.
type Eta struct {
	DurationS      float64 `json:"duration_s"`
	DistanceM      float64 `json:"distance_m"`
	EstimateSource string  `json:"estimate_source"`
}
