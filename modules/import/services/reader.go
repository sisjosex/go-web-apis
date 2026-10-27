package services

import (
	"bytes"
	"io"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/xuri/excelize/v2"
	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"

	importErrors "josex/web/modules/import/errors"
)

// sheetFile is an uploaded file read into its heading line and data rows, the
// same shape whether it came as CSV or as an Excel workbook (IMPORT-002 D1).
// Headers are the headings as written, trimmed; a record's cells follow them.
type sheetFile struct {
	sheet   string
	headers []string
	records [][]string
}

// zipMagic opens every .xlsx: a workbook is a zip archive.
var zipMagic = []byte("PK\x03\x04")

// isWorkbook tells an Excel file from a CSV by its name or, for a client that
// names it otherwise, by the zip signature a CSV never starts with.
func isWorkbook(data []byte, fileName string) bool {
	return strings.EqualFold(filepath.Ext(fileName), ".xlsx") || bytes.HasPrefix(data, zipMagic)
}

// readFile reads the heading line and at most maxRows+1 data rows — one past the
// limit is enough to refuse the file without holding all of it.
func readFile(data []byte, fileName string, maxRows int) (*sheetFile, error) {
	if len(data) == 0 {
		return nil, importErrors.ErrFileEmpty
	}
	if isWorkbook(data, fileName) {
		return readWorkbook(data, maxRows)
	}
	return readCSV(data, maxRows)
}

func readCSV(data []byte, maxRows int) (*sheetFile, error) {
	reader := newCSVReader(data)
	header, err := reader.Read()
	if err == io.EOF {
		return nil, importErrors.ErrFileEmpty
	}
	if err != nil {
		return nil, importErrors.ErrFileInvalid
	}
	file := &sheetFile{headers: trimAll(header)}
	for len(file.records) <= maxRows {
		record, readErr := reader.Read()
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return nil, importErrors.ErrFileInvalid
		}
		file.records = append(file.records, trimAll(record))
	}
	return file, nil
}

// readWorkbook streams the first sheet with a non-empty row (D1): row by row,
// never the whole sheet in memory. Cells come as Excel shows them, so a number
// formatted "1.500,50" reaches normalizeCells like the same cell saved as CSV.
func readWorkbook(data []byte, maxRows int) (*sheetFile, error) {
	book, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		return nil, importErrors.ErrFileInvalid
	}
	defer func() { _ = book.Close() }()

	for _, sheet := range book.GetSheetList() {
		file, err := readSheet(book, sheet, maxRows)
		if err != nil {
			return nil, err
		}
		if file != nil {
			return file, nil
		}
	}
	return nil, importErrors.ErrFileEmpty
}

// readSheet answers nil for a sheet without a single filled cell.
func readSheet(book *excelize.File, sheet string, maxRows int) (*sheetFile, error) {
	rows, err := book.Rows(sheet)
	if err != nil {
		return nil, importErrors.ErrFileInvalid
	}
	defer func() { _ = rows.Close() }()

	var file *sheetFile
	for rows.Next() {
		cells, err := rows.Columns()
		if err != nil {
			return nil, importErrors.ErrFileInvalid
		}
		cells = trimAll(cells)
		if file == nil {
			if isBlank(cells) {
				continue // a title row left empty above the headings
			}
			file = &sheetFile{sheet: sheet, headers: cells}
			continue
		}
		if isBlank(cells) {
			continue
		}
		file.records = append(file.records, cells)
		if len(file.records) > maxRows {
			break
		}
	}
	return file, nil
}

func trimAll(cells []string) []string {
	out := make([]string, len(cells))
	for i, cell := range cells {
		out[i] = strings.TrimSpace(cell)
	}
	return out
}

func isBlank(cells []string) bool {
	for _, cell := range cells {
		if cell != "" {
			return false
		}
	}
	return true
}

// foldHeading is how a heading, a field key and an alias are compared: lower
// case, accents dropped, every run of punctuation, underscores included, one
// space — "Cód. Art.", "cod_art" and "COD ART" are the same heading.
func foldHeading(value string) string {
	stripped, _, err := transform.String(transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC), value)
	if err != nil {
		stripped = value
	}
	var b strings.Builder
	space := false
	for _, r := range strings.ToLower(stripped) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			if space && b.Len() > 0 {
				b.WriteByte(' ')
			}
			space = false
			b.WriteRune(r)
			continue
		}
		space = true
	}
	return b.String()
}
