package push

import (
	"regexp"
	"strconv"

	"josex/web/modules/core/utils"
)

// noticeArg is an Android-style positional argument, `%2$s`: the same strings the phone holds, so the
// two read alike word for word.
var noticeArg = regexp.MustCompile(`%(\d+)\$s`)

// NoticeText writes a notice's title and body in lang (TENANCY-005 D1), the recipient's own language
// (`auth.users.locale`), from `notice.<type>.title` / `notice.<type>.body` in tracking's lang files. It
// answers empty strings for a type with no text there; the message then carries the loc keys instead.
// Map lookups and a replace: no I/O.
func NoticeText(lang, noticeType string, args []string) (title, body string) {
	titleKey, bodyKey := "notice."+noticeType+".title", "notice."+noticeType+".body"
	title = utils.GetTranslation(lang, titleKey)
	if title == titleKey {
		return "", ""
	}
	body = utils.GetTranslation(lang, bodyKey)
	if body == bodyKey {
		body = ""
	}
	return fill(title, args), fill(body, args)
}

// fill puts args in their positions; a position past the args is left empty, an unused arg ignored.
func fill(text string, args []string) string {
	return noticeArg.ReplaceAllStringFunc(text, func(arg string) string {
		n, _ := strconv.Atoi(noticeArg.FindStringSubmatch(arg)[1])
		if n < 1 || n > len(args) {
			return ""
		}
		return args[n-1]
	})
}
