package models

// RegisterPushDeviceDto is one phone's FCM registration (TRACK-012): the app's own install id, the
// platform, and the token FCM gave that install.
type RegisterPushDeviceDto struct {
	DeviceID string `json:"device_id" binding:"required,max=100"`
	Platform string `json:"platform" binding:"required,oneof=android ios"`
	Token    string `json:"token" binding:"required,max=4096"`
}
