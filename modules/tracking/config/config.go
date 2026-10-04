package config

import (
	"josex/web/modules/core/utils"
	"josex/web/modules/tracking/models"
	"time"
)

// TrackingConfig holds tracking module configuration
type TrackingConfig struct {
	// GPS location update settings
	MaxLocationBatchSize       int
	LocationRetentionDays      int
	LocationUpdateRateLimitSec int

	// DefaultTimezone is the zone a route or organization gets when the form sends none (TRACK-038 D4).
	DefaultTimezone string

	// Real-time notification settings
	EnableRealtimeNotifications bool
	NotificationDelayThreshold  time.Duration

	// Alert settings
	AlertAutoResolveHours    int
	EnableAlertNotifications bool

	// Performance settings
	EnableLocationPartitioning bool
	MaxEventsPerRider          int

	// Compliance document scans (TRACK-016 D3): the biggest one a ticket signs and a claim accepts.
	// They live in the private documents bucket (INFRA-007).
	MaxDocumentKB int

	// GPS pipeline (TRACK-010 D2): the stream is trimmed to about GPSStreamMaxLen points per tenant,
	// and a point within GPSArrivalRadiusM of the trip's next stop marks it arrived.
	GPSStreamMaxLen   int64
	GPSArrivalRadiusM int
	// Within ApproachRadiusM of the next stop a trip announces it to its riders once (TRACK-012 D4).
	ApproachRadiusM int

	// WebSocket gateway (TRACK-025): a ping every WSPingSeconds, and a connection closed 1013 once
	// WSQueueMax frames that must all arrive are waiting for it.
	WSPingSeconds int
	WSQueueMax    int

	// Push (TRACK-012 D2): the Firebase project and its service-account key. Either unset and notices
	// only reach the in-app feed.
	FCMProjectID       string
	FCMCredentialsFile string
}

// LoadTrackingConfig loads configuration from environment variables
func LoadTrackingConfig() *TrackingConfig {
	return &TrackingConfig{
		// GPS settings
		MaxLocationBatchSize:       utils.GetEnvAsInt("TRACKING_MAX_LOCATION_BATCH_SIZE", 100),
		LocationRetentionDays:      utils.GetEnvAsInt("TRACKING_LOCATION_RETENTION_DAYS", 2),
		LocationUpdateRateLimitSec: utils.GetEnvAsInt("TRACKING_LOCATION_RATE_LIMIT_SEC", 5),
		DefaultTimezone:            utils.GetEnv("TRACKING_DEFAULT_TIMEZONE", "America/La_Paz"),

		// Notification settings
		EnableRealtimeNotifications: utils.GetEnvAsBool("TRACKING_ENABLE_REALTIME_NOTIFICATIONS", true),
		NotificationDelayThreshold:  utils.GetEnvAsDuration("TRACKING_NOTIFICATION_DELAY_THRESHOLD", 15*time.Minute),

		// Alert settings
		AlertAutoResolveHours:    utils.GetEnvAsInt("TRACKING_ALERT_AUTO_RESOLVE_HOURS", 24),
		EnableAlertNotifications: utils.GetEnvAsBool("TRACKING_ENABLE_ALERT_NOTIFICATIONS", true),

		// Performance settings
		EnableLocationPartitioning: utils.GetEnvAsBool("TRACKING_ENABLE_LOCATION_PARTITIONING", true),
		MaxEventsPerRider:          utils.GetEnvAsInt("TRACKING_MAX_EVENTS_PER_RIDER", 1000),

		// Compliance documents
		MaxDocumentKB: utils.GetEnvAsInt("TRACKING_MAX_DOCUMENT_KB", 5120),

		// GPS pipeline
		GPSStreamMaxLen:   int64(utils.GetEnvAsInt("GPS_STREAM_MAXLEN", 100000)),
		GPSArrivalRadiusM: utils.GetEnvAsInt("GPS_ARRIVAL_RADIUS_M", 50),
		ApproachRadiusM:   utils.GetEnvAsInt("TRACKING_APPROACH_RADIUS_M", 800),

		// WebSocket gateway
		WSPingSeconds: utils.GetEnvAsInt("WS_PING_SECONDS", 25),
		WSQueueMax:    utils.GetEnvAsInt("WS_QUEUE_MAX", 64),

		// Push
		FCMProjectID:       utils.GetEnv("FCM_PROJECT_ID", ""),
		FCMCredentialsFile: utils.GetEnv("FCM_CREDENTIALS_FILE", ""),
	}
}

// IngestRadii are the two distances every stored GPS point is checked against.
func (c *TrackingConfig) IngestRadii() models.IngestRadii {
	return models.IngestRadii{ArrivalM: c.GPSArrivalRadiusM, ApproachM: c.ApproachRadiusM}
}
