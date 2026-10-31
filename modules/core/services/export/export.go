// Package export streams a list as a CSV download any module can reuse (CORE-003): rows go straight to the
// response, read in pages by the caller, so memory stays constant whatever the file's size.
package export

import (
	"encoding/csv"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	coreErrors "josex/web/modules/core/errors"
	"josex/web/modules/core/utils"

	"github.com/gin-gonic/gin"
)

// MaxRows is the most a download carries; past it the file ends with a row that says so (INV-021 D1).
const MaxRows = 50000

const flushEvery = 1000

// The codes a download can end with.
const (
	ErrFailed    = "export.failed"
	ErrTruncated = "export.truncated"
)

// bom makes Excel read the file as UTF-8, accents included.
var bom = []byte{0xEF, 0xBB, 0xBF}

var numeric = regexp.MustCompile(`^[+-]?\d+([.,]\d+)*$`)

// StreamCSV writes header and then each row next hands over, until it says there are no more. A failure
// before the first row answers 422 and no file; once the file has started the status is gone, so a
// failure, like passing MaxRows, ends it with one row that says what happened.
func StreamCSV(c *gin.Context, filename string, header []string, next func() ([]string, bool, error)) {
	row, more, err := next()
	if err != nil {
		c.AbortWithStatusJSON(http.StatusUnprocessableEntity, coreErrors.BuildErrorSingle(c, ErrFailed))
		return
	}

	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", `attachment; filename="`+strconv.Quote(filename)[1:])
	c.Status(http.StatusOK)
	if _, err := c.Writer.Write(bom); err != nil {
		return
	}
	out := csv.NewWriter(c.Writer)
	if err := out.Write(header); err != nil {
		return
	}

	lang := c.GetString("lang")
	cleaned := make([]string, 0, len(header))
	for rows := 0; more; rows++ {
		if rows == MaxRows {
			_ = out.Write([]string{utils.GetTranslation(lang, ErrTruncated)})
			break
		}
		cleaned = cleaned[:0]
		for _, cell := range row {
			cleaned = append(cleaned, Neutralize(cell))
		}
		if err := out.Write(cleaned); err != nil {
			return
		}
		if (rows+1)%flushEvery == 0 {
			out.Flush()
			c.Writer.Flush()
		}
		if row, more, err = next(); err != nil {
			_ = out.Write([]string{utils.GetTranslation(lang, ErrFailed)})
			break
		}
	}
	out.Flush()
	c.Writer.Flush()
}

// Neutralize stops a spreadsheet running a text cell as a formula: what starts with = + - @, a tab or a
// carriage return gets a leading apostrophe. A plain number, -5 included, stays a number.
func Neutralize(cell string) string {
	if cell == "" || !strings.ContainsRune("=+-@\t\r", rune(cell[0])) || numeric.MatchString(cell) {
		return cell
	}
	return "'" + cell
}
