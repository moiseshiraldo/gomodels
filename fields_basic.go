package gomodels

import (
	"database/sql"
	"database/sql/driver"
	"fmt"
)

type CharChoice struct {
	Value string
	Label string
}

type CharField struct {
	Null         bool         `json:",omitempty"`
	Blank        bool         `json:",omitempty"`
	Choices      []CharChoice `json:",omitempty"`
	Column       string       `json:",omitempty"`
	Index        bool         `json:",omitempty"`
	Default      string       `json:",omitempty"`
	DefaultEmpty bool         `json:",omitempty"`
	PrimaryKey   bool         `json:",omitempty"`
	Unique       bool         `json:",omitempty"`
	MaxLength    int          `json:",omitempty"`
}

func (f CharField) IsPK() bool {
	return f.PrimaryKey
}

func (f CharField) IsUnique() bool {
	return f.Unique
}

func (f CharField) IsNull() bool {
	return f.Null
}

func (f CharField) IsAuto() bool {
	return false
}

func (f CharField) IsAutoNow() bool {
	return false
}

func (f CharField) IsAutoNowAdd() bool {
	return false
}

func (f CharField) HasIndex() bool {
	return f.Index && !(f.PrimaryKey || f.Unique)
}

func (f CharField) DBColumn(name string) string {
	if f.Column != "" {
		return f.Column
	}
	return name
}

func (f CharField) DataType(driver string) string {
	return fmt.Sprintf("VARCHAR(%d)", f.MaxLength)
}

func (f CharField) DefaultVal() (Value, bool) {
	if f.Default != "" || f.DefaultEmpty {
		return f.Default, true
	} else {
		return nil, false
	}
}

func (f CharField) Recipient() interface{} {
	if f.Null {
		var val sql.NullString
		return &val
	}
	var val string
	return &val
}

func (f CharField) Value(rec interface{}) Value {
	if val, ok := rec.(sql.NullString); ok {
		if !val.Valid {
			return nil
		}
		return val.String
	}
	return rec
}

func (f CharField) DriverValue(val Value, dvr string) (interface{}, error) {
	if vlr, ok := val.(driver.Valuer); ok {
		return vlr.Value()
	}
	return val, nil
}

type BooleanField struct {
	Null         bool   `json:",omitempty"`
	Blank        bool   `json:",omitempty"`
	Column       string `json:",omitempty"`
	Index        bool   `json:",omitempty"`
	Default      bool   `json:",omitempty"`
	DefaultFalse bool   `json:",omitempty"`
}

func (f BooleanField) IsPK() bool {
	return false
}

func (f BooleanField) IsUnique() bool {
	return false
}

func (f BooleanField) IsNull() bool {
	return f.Null
}

func (f BooleanField) IsAuto() bool {
	return false
}

func (f BooleanField) IsAutoNow() bool {
	return false
}

func (f BooleanField) IsAutoNowAdd() bool {
	return false
}

func (f BooleanField) HasIndex() bool {
	return f.Index
}

func (f BooleanField) DBColumn(name string) string {
	if f.Column != "" {
		return f.Column
	}
	return name
}

func (f BooleanField) DataType(dvr string) string {
	return "BOOLEAN"
}

func (f BooleanField) DefaultVal() (Value, bool) {
	if f.Default {
		return true, true
	} else if f.DefaultFalse {
		return false, true
	} else {
		return nil, false
	}
}

func (f BooleanField) Recipient() interface{} {
	if f.Null {
		var val sql.NullBool
		return &val
	}
	var val bool
	return &val
}

func (f BooleanField) Value(rec interface{}) Value {
	if val, ok := rec.(sql.NullBool); ok {
		if !val.Valid {
			return nil
		}
		return val.Bool
	}
	return rec
}

func (f BooleanField) DriverValue(v Value, dvr string) (interface{}, error) {
	if vlr, ok := v.(driver.Valuer); ok {
		return vlr.Value()
	}
	return v, nil
}

// TODO: remove for Golang 1.13
type NullInt32 struct {
	Int32 int32
	Valid bool // Valid is true if Int32 is not NULL
}

// Scan implements the Scanner interface.
func (n *NullInt32) Scan(value interface{}) error {
	if value == nil {
		n.Int32, n.Valid = 0, false
		return nil
	}
	n.Valid = true
	return setRecipient(&n.Int32, value)
}

// Value implements the driver Valuer interface.
func (n NullInt32) Value() (driver.Value, error) {
	if !n.Valid {
		return nil, nil
	}
	return int64(n.Int32), nil
}

type IntChoice struct {
	Value int
	Label string
}

type IntegerField struct {
	Null        bool        `json:",omitempty"`
	Blank       bool        `json:",omitempty"`
	Choices     []IntChoice `json:",omitempty"`
	Column      string      `json:",omitempty"`
	Index       bool        `json:",omitempty"`
	Default     int32       `json:",omitempty"`
	DefaultZero bool        `json:",omitempty"`
	PrimaryKey  bool        `json:",omitempty"`
	Unique      bool        `json:",omitempty"`
	Auto        bool        `json:",omitempty"`
}

func (f IntegerField) IsPK() bool {
	return f.PrimaryKey
}

func (f IntegerField) IsUnique() bool {
	return f.Unique
}

func (f IntegerField) IsNull() bool {
	return f.Null
}

func (f IntegerField) IsAuto() bool {
	return f.Auto
}

func (f IntegerField) IsAutoNow() bool {
	return false
}

func (f IntegerField) IsAutoNowAdd() bool {
	return false
}

func (f IntegerField) HasIndex() bool {
	return f.Index && !(f.PrimaryKey || f.Unique)
}

func (f IntegerField) DBColumn(name string) string {
	if f.Column != "" {
		return f.Column
	}
	return name
}

func (f IntegerField) DataType(dvr string) string {
	if dvr == "postgres" && f.IsAuto() {
		return "SERIAL"
	} else {
		return "INTEGER"
	}
}

func (f IntegerField) DefaultVal() (Value, bool) {
	if f.Default != 0 || f.DefaultZero {
		return f.Default, true
	} else {
		return nil, false
	}
}

func (f IntegerField) Recipient() interface{} {
	if f.Null {
		var val NullInt32
		return &val
	}
	var val int32
	return &val
}

func (f IntegerField) Value(rec interface{}) Value {
	if val, ok := rec.(NullInt32); ok {
		if !val.Valid {
			return nil
		}
		return val.Int32
	}
	return rec
}

func (f IntegerField) DriverValue(v Value, dvr string) (interface{}, error) {
	if vlr, ok := v.(driver.Valuer); ok {
		return vlr.Value()
	}
	return v, nil
}
