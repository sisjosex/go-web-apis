package pdf

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"testing"
	"time"

	"josex/web/modules/core/services/codes"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var update = flag.Bool("update", false, "rewrite the golden files")

var fixedAt = time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)

func golden(t *testing.T, name string, got []byte) {
	t.Helper()
	path := filepath.Join("testdata", name)
	if *update {
		require.NoError(t, os.MkdirAll("testdata", 0o755))
		require.NoError(t, os.WriteFile(path, got, 0o644))
	}
	want, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.True(t, bytes.Equal(want, got), "%s differs from its golden file: run go test ./modules/core/services/pdf -update and read the PDF", name)
}

func sampleReceipt() *Doc {
	d := Receipt(Roll80, "es").At(fixedAt)
	d.Text("Farmacia Ñandú", Style{Bold: true, Size: 11, Align: Center})
	d.Text("Av. América 123 · Cochabamba", Style{Align: Center})
	d.Line()
	for _, line := range []struct {
		name  string
		total float64
	}{{"Paracetamol 500 mg × 2 cajas de veinte comprimidos", 45}, {"Alcohol en gel", 1234.5}} {
		d.Columns(Cell{Text: line.name, Share: 0.7}, Cell{Text: d.Number(line.total), Share: 0.3, Style: Style{Align: Right}})
	}
	d.Line()
	d.Total("Total Bs", 1279.5)
	d.Space(2)
	d.QR("https://taypi24.com/pagar/abc", 30)
	d.Barcode(codes.Code128, "TICKET-0142", 50, 10)
	return d
}

func sampleLetter() *Doc {
	d := Letter("es").At(fixedAt)
	d.Text("Reporte de existencias", Style{Bold: true, Size: 14})
	d.Space(3)
	for i := 1; i <= 20; i++ {
		d.Columns(
			Cell{Text: "Producto número " + string(rune('A'+i)), Share: 0.6},
			Cell{Text: "Almacén Central", Share: 0.2},
			Cell{Text: d.Number(float64(i) * 1000.5), Share: 0.2, Style: Style{Align: Right}},
		)
	}
	d.Line()
	d.Total("Total", 210105)
	return d
}

func TestReceipt_Golden(t *testing.T) {
	data, err := sampleReceipt().Bytes()

	require.NoError(t, err)
	golden(t, "receipt.pdf", data)
	assert.Less(t, len(data), 60*1024)
}

func TestLetter_Golden(t *testing.T) {
	data, err := sampleLetter().Bytes()

	require.NoError(t, err)
	golden(t, "letter.pdf", data)
	assert.Less(t, len(data), 60*1024)
}

func TestReceipt_NarrowRoll(t *testing.T) {
	data, err := Receipt(Roll58, "es").At(fixedAt).Text("Ticket", Style{}).Bytes()

	require.NoError(t, err)
	assert.True(t, bytes.HasPrefix(data, []byte("%PDF-")))
}

func TestDoc_InvalidBarcodeIsTheError(t *testing.T) {
	_, err := Letter("es").Barcode(codes.EAN13, "123", 50, 10).Bytes()

	assert.ErrorIs(t, err, codes.ErrInvalidData)
}

func TestNumber_FollowsTheLocale(t *testing.T) {
	es, en := Letter("es"), Letter("en")

	assert.Equal(t, "1.234.567,50", es.Number(1234567.5))
	assert.Equal(t, "1,234,567.50", en.Number(1234567.5))
	assert.Equal(t, "45,00", es.Number(45))
	assert.Equal(t, "-1.000,00", es.Number(-1000))
}

// The budget CORE-003 step 1 picked the library by: 100 ms and 60 KB a document.
func BenchmarkReceipt(b *testing.B) {
	for i := 0; i < b.N; i++ {
		if _, err := sampleReceipt().Bytes(); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkLetter(b *testing.B) {
	for i := 0; i < b.N; i++ {
		if _, err := sampleLetter().Bytes(); err != nil {
			b.Fatal(err)
		}
	}
}
