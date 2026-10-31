// Package pdf is the shared document renderer (CORE-003): a receipt on an 80 or 58 mm roll, or a letter page,
// built from a few blocks. Each module draws its own document with them. It uses the PDF standard fonts,
// which no file has to embed and which cover Spanish (Windows-1252), so a document stays a few KB.
package pdf

import (
	"bytes"
	"strconv"
	"strings"
	"sync"
	"time"

	"josex/web/modules/core/services/codes"

	"github.com/go-pdf/fpdf"
)

// Roll widths a receipt prints on, in mm.
const (
	Roll80 = 80.0
	Roll58 = 58.0
)

const (
	family     = "Helvetica"
	lineFactor = 0.42 // mm of line height per font point
)

// Align is where a block sits across the width.
type Align string

// The three alignments a block takes.
const (
	Left   Align = "L"
	Center Align = "C"
	Right  Align = "R"
)

// Style is how a text is set. A zero style is the document's default size, regular, left.
type Style struct {
	Size  float64
	Bold  bool
	Align Align
}

// Cell is one column of a row: Share is its part of the width, the shares of a row summing to 1.
type Cell struct {
	Text  string
	Share float64
	Style Style
}

// buffers keeps the output buffers: a busy till renders a receipt per sale.
var buffers = sync.Pool{New: func() any { return new(bytes.Buffer) }}

// Doc collects blocks and draws them on Bytes. A receipt's page height is the sum of its blocks, so
// nothing can be drawn until all of them are known; collecting does the same job for a letter.
type Doc struct {
	f           *fpdf.Fpdf
	tr          func(string) string
	width       float64
	margin      float64
	defaultSize float64
	roll        bool
	locale      string
	blocks      []block
	images      int
	now         time.Time
	err         error
}

// block is one thing to draw and the height it takes on the page.
type block struct {
	height float64
	draw   func()
}

// Receipt starts a document for a roll of width mm. locale ("es" or "en") sets the number format.
func Receipt(width float64, locale string) *Doc {
	margin, size := 3.0, 8.5
	if width <= Roll58 {
		margin, size = 2.0, 7.5
	}
	f := fpdf.NewCustom(&fpdf.InitType{OrientationStr: "P", UnitStr: "mm", Size: fpdf.SizeType{Wd: width, Ht: 297}})
	return newDoc(f, width-2*margin, margin, size, true, locale)
}

// Letter starts a document on letter pages that break by themselves.
func Letter(locale string) *Doc {
	f := fpdf.NewCustom(&fpdf.InitType{OrientationStr: "P", UnitStr: "mm", SizeStr: "Letter"})
	pageWidth, _ := f.GetPageSize()
	const margin = 18.0
	return newDoc(f, pageWidth-2*margin, margin, 10, false, locale)
}

func newDoc(f *fpdf.Fpdf, width, margin, size float64, roll bool, locale string) *Doc {
	f.SetMargins(margin, margin, margin)
	f.SetAutoPageBreak(!roll, margin)
	f.SetCatalogSort(true) // fonts and images in a fixed order: one input, one output
	f.SetFont(family, "", size)
	return &Doc{f: f, tr: f.UnicodeTranslatorFromDescriptor(""), width: width, margin: margin, defaultSize: size, roll: roll, locale: locale, now: time.Now()}
}

// At fixes the document's dates, so one input always renders the same bytes.
func (d *Doc) At(now time.Time) *Doc {
	d.now = now
	return d
}

// Number is value with 2 decimals in the document's locale: 1.234,50 in Spanish, 1,234.50 in English.
func (d *Doc) Number(value float64) string {
	digits := strconv.FormatFloat(value, 'f', 2, 64)
	sign := ""
	if strings.HasPrefix(digits, "-") {
		sign, digits = "-", digits[1:]
	}
	whole, cents, _ := strings.Cut(digits, ".")
	thousands, decimal := ".", ","
	if d.locale == "en" {
		thousands, decimal = ",", "."
	}
	var grouped strings.Builder
	for i, digit := range whole {
		if i > 0 && (len(whole)-i)%3 == 0 {
			grouped.WriteString(thousands)
		}
		grouped.WriteRune(digit)
	}
	return sign + grouped.String() + decimal + cents
}

// Text adds a paragraph, wrapped to the width.
func (d *Doc) Text(text string, style Style) *Doc {
	return d.Columns(Cell{Text: text, Share: 1, Style: style})
}

// Columns adds one row. A cell too long for its share wraps, and the row takes its tallest cell.
func (d *Doc) Columns(cells ...Cell) *Doc {
	lines := make([][]string, len(cells))
	height := 0.0
	for i := range cells {
		size := d.size(cells[i].Style)
		d.f.SetFont(family, weight(cells[i].Style), size)
		lines[i] = d.wrap(cells[i].Text, cells[i].Share*d.width)
		height = max(height, float64(len(lines[i]))*size*lineFactor)
	}
	d.blocks = append(d.blocks, block{height: height, draw: func() {
		x, y := d.f.GetX(), d.f.GetY()
		for i, cell := range cells {
			size := d.size(cell.Style)
			d.f.SetFont(family, weight(cell.Style), size)
			d.f.SetXY(x, y)
			cellWidth := cell.Share * d.width
			for _, line := range lines[i] {
				d.f.SetX(x)
				d.f.CellFormat(cellWidth, size*lineFactor, line, "", 2, string(alignOf(cell.Style)), false, 0, "")
			}
			x += cellWidth
		}
		d.f.SetXY(d.margin, y+height)
	}})
	return d
}

// wrap breaks the encoded text into lines of width mm. It measures bytes, not runes: Windows-1252 text is
// not valid UTF-8, which is why fpdf's own SplitText cannot read it.
func (d *Doc) wrap(text string, width float64) []string {
	lines := []string{}
	current := ""
	for _, word := range strings.Fields(d.tr(text)) {
		for d.f.GetStringWidth(word) > width && len(word) > 1 {
			if current != "" {
				lines = append(lines, current)
				current = ""
			}
			cut := len(word) - 1
			for cut > 1 && d.f.GetStringWidth(word[:cut]) > width {
				cut--
			}
			lines = append(lines, word[:cut])
			word = word[cut:]
		}
		switch {
		case current == "":
			current = word
		case d.f.GetStringWidth(current+" "+word) <= width:
			current += " " + word
		default:
			lines = append(lines, current)
			current = word
		}
	}
	return append(lines, current)
}

// Total adds a label on the left and an amount on the right, bold.
func (d *Doc) Total(label string, value float64) *Doc {
	bold := Style{Bold: true, Size: d.defaultSize + 1}
	right := bold
	right.Align = Right
	return d.Columns(Cell{Text: label, Share: 0.6, Style: bold}, Cell{Text: d.Number(value), Share: 0.4, Style: right})
}

// Line adds a thin rule across the width.
func (d *Doc) Line() *Doc {
	const gap = 1.5
	d.blocks = append(d.blocks, block{height: 2 * gap, draw: func() {
		y := d.f.GetY() + gap
		d.f.SetDrawColor(120, 120, 120)
		d.f.SetLineWidth(0.15)
		d.f.Line(d.margin, y, d.margin+d.width, y)
		d.f.SetXY(d.margin, y+gap)
	}})
	return d
}

// Space adds empty height, in mm.
func (d *Doc) Space(height float64) *Doc {
	d.blocks = append(d.blocks, block{height: height, draw: func() {
		d.f.SetXY(d.margin, d.f.GetY()+height)
	}})
	return d
}

// Image adds a PNG centred, width mm wide, keeping its proportions.
func (d *Doc) Image(pngBytes []byte, width float64) *Doc {
	d.images++
	name := "img" + strconv.Itoa(d.images)
	options := fpdf.ImageOptions{ImageType: "PNG"}
	info := d.f.RegisterImageOptionsReader(name, options, bytes.NewReader(pngBytes))
	if info == nil || d.f.Err() {
		d.err = d.f.Error()
		return d
	}
	height := width * info.Height() / info.Width()
	d.blocks = append(d.blocks, block{height: height, draw: func() {
		y := d.f.GetY()
		d.f.ImageOptions(name, d.margin+(d.width-width)/2, y, width, height, false, options, 0, "")
		d.f.SetXY(d.margin, y+height)
	}})
	return d
}

// QR adds a QR code of size mm, centred.
func (d *Doc) QR(data string, size float64) *Doc {
	image, err := codes.QR(data, int(size*12))
	if err != nil {
		d.err = err
		return d
	}
	return d.Image(image, size)
}

// Barcode adds a barcode of width by height mm, centred.
func (d *Doc) Barcode(symbology, data string, width, height float64) *Doc {
	image, err := codes.Barcode(symbology, data, int(width*12), int(height*12))
	if err != nil {
		d.err = err
		return d
	}
	return d.Image(image, width)
}

// Bytes draws every block and answers the PDF.
func (d *Doc) Bytes() ([]byte, error) {
	if d.err != nil {
		return nil, d.err
	}
	d.f.SetCreationDate(d.now)
	d.f.SetModificationDate(d.now)
	if d.roll {
		total := 2 * d.margin
		for _, b := range d.blocks {
			total += b.height
		}
		pageWidth, _ := d.f.GetPageSize()
		d.f.AddPageFormat("P", fpdf.SizeType{Wd: pageWidth, Ht: total})
	} else {
		d.f.AddPage()
	}
	for _, b := range d.blocks {
		b.draw()
	}
	buf := buffers.Get().(*bytes.Buffer)
	buf.Reset()
	defer buffers.Put(buf)
	if err := d.f.Output(buf); err != nil {
		return nil, err
	}
	return bytes.Clone(buf.Bytes()), nil
}

func (d *Doc) size(style Style) float64 {
	if style.Size > 0 {
		return style.Size
	}
	return d.defaultSize
}

func weight(style Style) string {
	if style.Bold {
		return "B"
	}
	return ""
}

func alignOf(style Style) Align {
	if style.Align == "" {
		return Left
	}
	return style.Align
}
