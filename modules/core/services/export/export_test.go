package export

import (
	"bytes"
	"encoding/csv"
	"errors"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strconv"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// countingWriter drops the bytes: what grows under the stream is the stream's own memory, not this buffer.
type countingWriter struct {
	httptest.ResponseRecorder
	size int
}

func (w *countingWriter) Write(b []byte) (int, error) {
	w.size += len(b)
	return len(b), nil
}

func (w *countingWriter) Flush() {}

func heap() uint64 {
	runtime.GC()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return m.HeapAlloc
}

func TestStreamCSV_120000RowsConstantMemoryCutAtTheCap(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := &countingWriter{ResponseRecorder: *httptest.NewRecorder()}
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/export", nil)
	asked := 0
	var peak uint64
	before := heap()

	StreamCSV(c, "big.csv", []string{"a", "b"}, func() ([]string, bool, error) {
		asked++
		if asked%10000 == 0 {
			peak = max(peak, heap())
		}
		return []string{"producto " + strconv.Itoa(asked), "12,50"}, asked <= 120000, nil
	})

	assert.Equal(t, MaxRows+1, asked, "reading stops at the cap")
	assert.Less(t, peak-min(peak, before), uint64(10<<20), "memory growth while streaming")
	assert.Greater(t, w.size, MaxRows*10)
}

func TestStreamCSV_FileShape(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/export", nil)
	rows := [][]string{{"=1+1", "-5"}, {"Polera, básica", "3"}}
	i := -1

	StreamCSV(c, "stock.csv", []string{"Nombre", "Cantidad"}, func() ([]string, bool, error) {
		i++
		if i == len(rows) {
			return nil, false, nil
		}
		return rows[i], true, nil
	})

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Disposition"), `attachment; filename="stock.csv"`)
	assert.Equal(t, "text/csv; charset=utf-8", w.Header().Get("Content-Type"))
	body := w.Body.Bytes()
	require.True(t, bytes.HasPrefix(body, bom))
	read, err := csv.NewReader(bytes.NewReader(body[len(bom):])).ReadAll()
	require.NoError(t, err)
	assert.Equal(t, [][]string{{"Nombre", "Cantidad"}, {"'=1+1", "-5"}, {"Polera, básica", "3"}}, read)
}

func TestStreamCSV_FailureBeforeTheFirstRowIs422(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/export", nil)

	StreamCSV(c, "x.csv", []string{"a"}, func() ([]string, bool, error) { return nil, false, errors.New("boom") })

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
	assert.Contains(t, w.Body.String(), ErrFailed)
}

func TestStreamCSV_FailureMidFileEndsWithARow(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/export", nil)
	calls := 0

	StreamCSV(c, "x.csv", []string{"a"}, func() ([]string, bool, error) {
		calls++
		if calls == 2 {
			return nil, false, errors.New("boom")
		}
		return []string{"ok"}, true, nil
	})

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), ErrFailed)
}

func TestNeutralize(t *testing.T) {
	for in, want := range map[string]string{
		"=1+1":    "'=1+1",
		"+cmd":    "'+cmd",
		"-1+1":    "'-1+1",
		"@SUM(A)": "'@SUM(A)",
		"\tx":     "'\tx",
		"\rx":     "'\rx",
		"-5":      "-5",
		"+5":      "+5",
		"-5,25":   "-5,25",
		"Polera":  "Polera",
		"":        "",
	} {
		assert.Equal(t, want, Neutralize(in), in)
	}
}
