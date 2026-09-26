package errors

// Geo module error codes (INFRA-003)
const (
	ValidationFailed = "geo.validation.failed"
	// PointIncomplete refuses a lat without a lng or the other way round.
	PointIncomplete = "geo.point.incomplete"

	GeocodeFailed   = "geo.geocode.failed"
	ReverseNotFound = "geo.reverse.not-found"
	ReverseFailed   = "geo.reverse.failed"

	// RouteUnavailable is Valhalla down, slow or behind an open breaker — retry later.
	RouteUnavailable = "geo.route.unavailable"
	// RouteNotFound is Valhalla answering that no route joins the locations.
	RouteNotFound = "geo.route.not-found"
	EtaFailed     = "geo.eta.failed"
)
