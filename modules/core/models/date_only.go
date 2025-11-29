package models

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"
)

type DateOnly time.Time

const dateFormat = "2006-01-02"

// UnmarshalJSON para convertir de JSON a DateOnly
func (d *DateOnly) UnmarshalJSON(bs []byte) error {
	var s string
	err := json.Unmarshal(bs, &s)
	if err != nil {
		return err
	}
	t, err := time.ParseInLocation(dateFormat, s, time.UTC)
	if err != nil {
		return err
	}
	*d = DateOnly(t)
	return nil
}

// MarshalJSON para convertir de DateOnly a JSON
func (d DateOnly) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Time(d).Format(dateFormat))
}

// Implementación de la interfaz driver.Valuer para convertir DateOnly a un valor manejable por PostgreSQL
func (d DateOnly) Value() (driver.Value, error) {
	return time.Time(d).Format(dateFormat), nil
}

// Implementación de la interfaz sql.Scanner para leer valores de PostgreSQL
func (d *DateOnly) Scan(value interface{}) error {
	if value == nil {
		return errors.New("received nil value")
	}

	switch v := value.(type) {
	case time.Time:
		*d = DateOnly(v)
		return nil
	default:
		return errors.New("unsupported type")
	}
}
