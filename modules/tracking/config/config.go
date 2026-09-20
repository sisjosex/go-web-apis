package config

import (
	"josex/web/modules/core/utils"
	"time"
)

// TrackingConfig holds tracking module configuration
type TrackingConfig struct {
	// GPS location update settings
	MaxLocationBatchSize       int
	LocationRetentionDays      int
	LocationUpdateRateLimitSec int

	// Real-time notification settings
	EnableRealtimeNotifications bool
	NotificationDelayThreshold  time.Duration

	// Alert settings
	AlertAutoResolveHours    int
	EnableAlertNotifications bool

	// Performance settings
	EnableLocationPartitioning bool
	MaxEventsPerRider          int

	// Compliance document scans (TRACK-016 D3). DocumentsRoot is deliberately not under MEDIA_ROOT:
	// a licence scan is not public content and is only reachable through the download endpoint.
	DocumentsRoot string
	MaxDocumentKB int
}

// LoadTrackingConfig loads configuration from environment variables
func LoadTrackingConfig() *TrackingConfig {
	return &TrackingConfig{
		// GPS settings
		MaxLocationBatchSize:       utils.GetEnvAsInt("TRACKING_MAX_LOCATION_BATCH_SIZE", 100),
		LocationRetentionDays:      utils.GetEnvAsInt("TRACKING_LOCATION_RETENTION_DAYS", 90),
		LocationUpdateRateLimitSec: utils.GetEnvAsInt("TRACKING_LOCATION_RATE_LIMIT_SEC", 5),

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
		DocumentsRoot: utils.GetEnv("TRACKING_DOCUMENTS_ROOT", "storage/tracking-documents"),
		MaxDocumentKB: utils.GetEnvAsInt("TRACKING_MAX_DOCUMENT_KB", 5120),
	}
}
