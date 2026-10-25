package models

// The taypi go notices a workspace may switch off (TENANCY-007 D2, D3), declared in the module's
// settings.go: the notifier drops a switched-off notice's push; the trip's history keeps the event.
const (
	SettingNoticeBoarding    = "tracking.notice.boarding"
	SettingNoticeDelay       = "tracking.notice.delay"
	SettingNoticeApproaching = "tracking.notice.approaching"
)

// NoticeSetting is the setting that switches a notice type off, "" for one no setting governs.
func NoticeSetting(noticeType string) string {
	switch noticeType {
	case "boarded", "dropped_off":
		return SettingNoticeBoarding
	case "delay":
		return SettingNoticeDelay
	case "approaching":
		return SettingNoticeApproaching
	}
	return ""
}
