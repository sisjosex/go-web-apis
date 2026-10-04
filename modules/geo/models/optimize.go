package models

// The route-optimization problem and answer (TRACK-014 D1), engine-neutral. Times are seconds since
// midnight; every pickup takes one seat.

// OptimizeVehicle is one vehicle: it starts at its first pickup and ends at End, within TimeWindow,
// carrying at most Capacity, driving no longer than MaxTravelS in all (0: no limit).
type OptimizeVehicle struct {
	ID         int
	End        LatLng
	Capacity   int
	TimeWindow [2]int
	MaxTravelS int
}

// OptimizeJob is one pickup: a rider at Location, ServiceS seconds to board.
type OptimizeJob struct {
	ID       int
	Location LatLng
	ServiceS int
}

// OptimizeProblem is what is asked: the costing the roads are driven with, the vehicles and the jobs.
type OptimizeProblem struct {
	Costing  string
	Vehicles []OptimizeVehicle
	Jobs     []OptimizeJob
}

// OptimizeStop is one job of a route and when the vehicle reaches it.
type OptimizeStop struct {
	JobID    int
	ArrivalS int
}

// OptimizeRoute is one used vehicle: its stops in order, when it reaches its end, and how far and
// long it drives from the first stop.
type OptimizeRoute struct {
	VehicleID int
	Stops     []OptimizeStop
	EndS      int
	DistanceM float64
	DurationS int
}

// OptimizeSolution is the answer: the routes, and the jobs no vehicle could take.
type OptimizeSolution struct {
	Routes     []OptimizeRoute
	Unassigned []int
}
