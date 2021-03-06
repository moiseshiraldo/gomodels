package gomodel

import (
	"fmt"
	"reflect"
	"strings"
	"time"
)

// An Instance represents a particular model object and offers some methods
// to interact with its field values and the database.
type Instance struct {
	model     *Model
	container Container
}

// trace returns the ErrorTrace for the instance.
func (i Instance) trace(err error) ErrorTrace {
	return ErrorTrace{App: i.model.app, Model: i.model, Err: err}
}

// Container returns the container holding the values of this model object.
func (i Instance) Container() Container {
	return i.container
}

// Model returns the model represented by this object.
func (i Instance) Model() *Model {
	return i.model
}

// GetIf returns the value for the given field name, and a hasField boolean
// indicating if the field was actually found in the underlying container.
func (i Instance) GetIf(name string) (val Value, hasField bool) {
	if name == "pk" {
		name = i.model.pk
	}
	field, ok := i.model.fields[name]
	if !ok {
		return nil, false
	}
	val, ok = getContainerField(i.container, name)
	if !ok {
		return nil, false
	}
	return field.Value(val), true
}

// Get returns the value for the given field name, or nil if not found.
func (i Instance) Get(name string) Value {
	val, _ := i.GetIf(name)
	return val
}

// Display returns the string representation of the value for the given field
// name, blank if not found.
func (i Instance) Display(name string) string {
	if name == "pk" {
		name = i.model.pk
	}
	field, ok := i.model.fields[name]
	if !ok {
		return ""
	}
	val, ok := getContainerField(i.container, name)
	if !ok {
		return ""
	}
	return field.DisplayValue(val)
}

// Set updates the named instance field with the given value. The change doesn't
// propagate to the database unless the Save method is called.
func (i Instance) Set(name string, val Value) error {
	if name == "pk" {
		name = i.model.pk
	}
	field, ok := i.model.fields[name]
	if !ok {
		return &ContainerError{i.trace(fmt.Errorf("unknown field %s", name))}
	}
	if c, ok := i.container.(Setter); ok {
		if err := c.Set(name, val, field); err != nil {
			return &ContainerError{i.trace(err)}
		}
	} else {
		cv := reflect.Indirect(reflect.ValueOf(i.container))
		f := cv.FieldByName(strings.Title(name))
		if !f.IsValid() || !f.CanSet() || !f.CanAddr() {
			return &ContainerError{i.trace(fmt.Errorf("invalid field"))}
		}
		if err := setRecipient(f.Addr().Interface(), val); err != nil {
			return &ContainerError{i.trace(err)}
		}
	}
	return nil
}

// SetValues updates the instance with the given values. The changes don't
// propagate to the database unless the Save method is called.
func (i Instance) SetValues(values Container) error {
	for name := range i.model.fields {
		if val, ok := getContainerField(values, name); ok {
			if err := i.Set(name, val); err != nil {
				return &ContainerError{i.trace(err)}
			}
		}
	}
	return nil
}

// valueToSave returns the value to be saved on the db for the named field,
// and a boolean indicating if there's a value to save.
func (i Instance) valueToSave(name string, creating bool) (Value, bool, error) {
	field, ok := i.model.fields[name]
	if !ok {
		err := fmt.Errorf("unknown field: %s", name)
		return nil, false, err
	}
	if field.IsAuto() {
		return nil, false, nil
	}
	if field.IsAutoNow() || creating && field.IsAutoNowAdd() {
		val := time.Now()
		if err := i.Set(name, val); err != nil {
			return nil, false, err
		}
		return val, true, nil
	}
	if val, ok := getContainerField(i.container, name); ok {
		return val, true, nil
	} else if val, hasDefault := field.DefaultValue(); creating && hasDefault {
		if err := i.Set(name, val); err != nil {
			return nil, false, err
		}
		return val, true, nil
	}
	return nil, false, nil
}

// engine returns the Engine and database identifier for the given target,
// nil and blank for invalid target.
func (i Instance) engine(target interface{}) (Engine, string) {
	switch t := target.(type) {
	case *Transaction:
		return t.Engine, t.DB.id
	case string:
		if db, ok := dbRegistry[t]; ok {
			return db.Engine, t
		}
	}
	return nil, ""
}

// insertRow saves the given instance fields on db.
func (i Instance) insertRow(
	target interface{},
	autoPk bool,
	fields ...string,
) error {
	eng, dbName := i.engine(target)
	if eng == nil {
		return &DatabaseError{Trace: i.trace(fmt.Errorf("invalid target"))}
	}
	dbValues := Values{}
	for _, name := range fields {
		if val, ok, err := i.valueToSave(name, true); err != nil {
			return &ContainerError{i.trace(err)}
		} else if ok && val != nil {
			dbValues[name] = val
		}
	}
	pk, err := eng.InsertRow(i.model, dbValues)
	if err != nil {
		return &DatabaseError{dbName, i.trace(err)}
	}
	if autoPk {
		if err := i.Set("pk", pk); err != nil {
			return err
		}
	}
	return nil
}

// updateRow updates the given fields on db row matching pkVal.
func (i Instance) updateRow(
	target interface{},
	pkVal Value,
	fields ...string,
) error {
	eng, dbName := i.engine(target)
	if eng == nil {
		return &DatabaseError{Trace: i.trace(fmt.Errorf("invalid target"))}
	}
	dbValues := Values{}
	for _, name := range fields {
		if name == i.model.pk {
			continue
		}
		val, ok, err := i.valueToSave(name, false)
		if err != nil {
			return &ContainerError{i.trace(err)}
		} else if ok {
			dbValues[name] = val
		}
	}
	options := QueryOptions{Conditioner: Q{"pk": pkVal}}
	rows, err := eng.UpdateRows(i.model, dbValues, options)
	if err != nil {
		return &DatabaseError{dbName, i.trace(err)}
	}
	if rows == 0 {
		return i.insertRow(target, false, fields...)
	}
	return nil
}

// save propagates the values of the given fields to the given database target.
func (i Instance) save(target interface{}, fields ...string) error {
	if len(fields) == 0 {
		for name := range i.model.fields {
			fields = append(fields, name)
		}
	}
	autoPk := i.model.fields[i.model.pk].IsAuto()
	pkVal := i.Get("pk")
	if pkVal != nil {
		zero := reflect.Zero(reflect.TypeOf(pkVal)).Interface()
		if !(autoPk && pkVal == zero) {
			return i.updateRow(target, pkVal, fields...)
		}
	}
	return i.insertRow(target, autoPk, fields...)
}

// Save propagates the instance field values to the database. If no field names
// are provided, all fields will be saved.
//
// The method will try to update the row matching the instance pk. If no row is
// updated, a new one will be inserted.
//
// If the pk field is auto incremented and the pk has the zero value, a new
// row will be inserted.
func (i Instance) Save(fields ...string) error {
	return i.save("default", fields...)
}

// SaveOn works as Save, but the changes are propagated to the given target,
// that can be a *Transaction or a string representing a database identifier.
func (i Instance) SaveOn(target interface{}, fields ...string) error {
	return i.save(target, fields...)
}

// delete removes the object from the given database target.
func (i Instance) delete(target interface{}) error {
	eng, dbName := i.engine(target)
	if eng == nil {
		return &DatabaseError{dbName, i.trace(fmt.Errorf("invalid target"))}
	}
	pkVal, ok := i.GetIf("pk")
	if !ok {
		return &ContainerError{Trace: i.trace(fmt.Errorf("pk not found"))}
	}
	_, err := eng.DeleteRows(i.model, QueryOptions{Conditioner: Q{"pk": pkVal}})
	return err
}

// Delete removes the object from the table on the default database.
func (i Instance) Delete() error {
	return i.delete("default")
}

// Delete removes the object from the table on the given target, that can be a
// *Transaction or a string representing a database identifier.
func (i Instance) DeleteOn(target interface{}) error {
	return i.delete(target)
}
