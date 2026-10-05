package models

import "github.com/google/uuid"

// The rider places slice of TRACK-044 D4.

// RiderPlace is a place a rider is picked up at or taken to ("Casa", "Oficina"). The default one is the
// rider's home: its pin and address are the rider's home_latitude/home_longitude and address.
type RiderPlace struct {
	ID        uuid.UUID `json:"id"`
	Label     string    `json:"label"`
	Address   *string   `json:"address"`
	Latitude  float64   `json:"latitude"`
	Longitude float64   `json:"longitude"`
	IsDefault bool      `json:"is_default"`
}

// ListRiderPlacesResponse is GET /tracking/riders/:rider_id/places, the default first.
type ListRiderPlacesResponse struct {
	Places []*RiderPlace `json:"places"`
}

// CreateRiderPlaceDto is POST /tracking/riders/:rider_id/places. The rider's first place is the default
// whatever IsDefault says.
type CreateRiderPlaceDto struct {
	Label     string   `json:"label" binding:"required,max=60" conform:"trim"`
	Address   *string  `json:"address" binding:"omitempty,max=500" conform:"trim"`
	Latitude  *float64 `json:"latitude" binding:"required,min=-90,max=90"`
	Longitude *float64 `json:"longitude" binding:"required,min=-180,max=180"`
	IsDefault bool     `json:"is_default"`
}

// UpdateRiderPlaceDto is PATCH /tracking/riders/:rider_id/places/:place_id: a field left out keeps its
// value, the pin moves with both coordinates, and is_default true makes the place the rider's home.
type UpdateRiderPlaceDto struct {
	Label     *string  `json:"label" binding:"omitempty,min=1,max=60" conform:"trim"`
	Address   *string  `json:"address" binding:"omitempty,max=500" conform:"trim"`
	Latitude  *float64 `json:"latitude" binding:"required_with=Longitude,omitempty,min=-90,max=90"`
	Longitude *float64 `json:"longitude" binding:"required_with=Latitude,omitempty,min=-180,max=180"`
	IsDefault *bool    `json:"is_default"`
}
