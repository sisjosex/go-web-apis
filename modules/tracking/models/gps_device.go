package models

import "time"

// The vehicle page's GPS device card (TRACK-068): whether a tracker is linked, and the token it posts
// with, shown once when issued.

// GPSDeviceStatus is GET /vehicles/{vehicle_id}/gps-device.
type GPSDeviceStatus struct {
	Linked     bool       `json:"linked"`
	IssuedAt   *time.Time `json:"issued_at"`
	LastSeenAt *time.Time `json:"last_seen_at"`
}

// GPSDeviceIssued is the 201 of POST /vehicles/{vehicle_id}/gps-device: the token, answered this once.
type GPSDeviceIssued struct {
	Token    string    `json:"token"`
	IssuedAt time.Time `json:"issued_at"`
}
