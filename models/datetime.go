package models

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"
)

// DateTime wraps time.Time to serialize as RFC3339 (ISO 8601) in JSON
// Example: "2025-11-28T12:00:00Z"
type DateTime time.Time

const datetimeFormat = time.RFC3339

// MarshalJSON converts DateTime to RFC3339 JSON string
func (dt DateTime) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Time(dt).Format(datetimeFormat))
}

// UnmarshalJSON parses RFC3339 JSON string to DateTime
func (dt *DateTime) UnmarshalJSON(bs []byte) error {
	var s string
	err := json.Unmarshal(bs, &s)
	if err != nil {
		return err
	}
	t, err := time.Parse(datetimeFormat, s)
	if err != nil {
		return err
	}
	*dt = DateTime(t)
	return nil
}

// Value implements driver.Valuer for database storage
func (dt DateTime) Value() (driver.Value, error) {
	return time.Time(dt), nil
}

// Scan implements sql.Scanner for database retrieval
func (dt *DateTime) Scan(value interface{}) error {
	if value == nil {
		return errors.New("cannot scan NULL into DateTime")
	}

	switch v := value.(type) {
	case time.Time:
		*dt = DateTime(v)
		return nil
	default:
		return errors.New("unsupported type for DateTime")
	}
}
