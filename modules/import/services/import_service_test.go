package services

import (
	"errors"
	"testing"

	importErrors "josex/web/modules/import/errors"
)

// parseCSV distinguishes a file with nothing in it from a file whose header is
// present but carries no rows below it. Both used to return ErrFileEmpty, which
// told the operator of a 0-byte upload that it "has no data rows" (A2).

func TestParseCSV_EmptyBytesAreFileEmpty(t *testing.T) {
	_, _, err := parseCSV([]byte(""))

	if !errors.Is(err, importErrors.ErrFileEmpty) {
		t.Fatalf("expected ErrFileEmpty, got %v", err)
	}
}

func TestParseCSV_HeaderWithoutRowsIsNoRows(t *testing.T) {
	_, _, err := parseCSV([]byte("first_name,last_name,email\r\n"))

	if !errors.Is(err, importErrors.ErrFileNoRows) {
		t.Fatalf("expected ErrFileNoRows, got %v", err)
	}
}

func TestParseCSV_BlankLinesAfterHeaderAreNoRows(t *testing.T) {
	_, _, err := parseCSV([]byte("first_name,last_name,email\r\n\r\n\r\n"))

	if !errors.Is(err, importErrors.ErrFileNoRows) {
		t.Fatalf("expected ErrFileNoRows, got %v", err)
	}
}

func TestParseCSV_HeaderAndOneRowSucceeds(t *testing.T) {
	headers, rows, err := parseCSV([]byte("First_Name, email\r\nAna, ana@example.com\r\n"))

	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if len(headers) != 2 || headers[0] != "first_name" || headers[1] != "email" {
		t.Fatalf("expected normalized headers, got %v", headers)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	if rows[0].line != 2 {
		t.Errorf("expected the first data row on line 2, got %d", rows[0].line)
	}
	if rows[0].data["email"] != "ana@example.com" {
		t.Errorf("expected the value trimmed, got %q", rows[0].data["email"])
	}
}
