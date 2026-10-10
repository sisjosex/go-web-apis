package push

import (
	"testing"

	"josex/web/modules/core/utils"

	"github.com/stretchr/testify/assert"
)

// TENANCY-005 D1: a notice is written in its recipient's language, with Android's positional args.
func TestNoticeText_WritesInTheRecipientsLanguage(t *testing.T) {
	saved := utils.Translations
	t.Cleanup(func() { utils.Translations = saved })
	utils.Translations = map[string]map[string]string{
		"en": {"notice.boarded.title": "%1$s is on board", "notice.boarded.body": "%1$s got on %2$s at %3$s."},
		"es": {"notice.boarded.title": "%1$s subió", "notice.boarded.body": "%1$s subió a %2$s en %3$s."},
	}
	args := []string{"Ana", "Ruta mañana"}

	esTitle, esBody := NoticeText("es", "boarded", args)
	enTitle, _ := NoticeText("en", "boarded", args)
	noTitle, noBody := NoticeText("es", "unknown_type", args)

	assert.Equal(t, "Ana subió", esTitle)
	assert.Equal(t, "Ana subió a Ruta mañana en .", esBody, "a missing arg is left empty")
	assert.Equal(t, "Ana is on board", enTitle)
	assert.Equal(t, [2]string{"", ""}, [2]string{noTitle, noBody}, "no text: the loc keys go instead")
}
