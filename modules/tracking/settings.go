package tracking

import (
	"josex/web/modules/core/registry"
	"josex/web/modules/tracking/models"
)

func init() {
	for _, key := range []string{models.SettingNoticeBoarding, models.SettingNoticeDelay, models.SettingNoticeApproaching} {
		registry.RegisterSetting(registry.SettingSpec{Key: key, Module: "tracking", Type: registry.SettingBool, Default: true})
	}
}
