package gomodels

import (
	"fmt"
	"reflect"
	"strings"
)

type QuerySet interface {
	Load() ([]*Instance, error)
	Query() (query string, values []interface{})
	Model() *Model
	Container() Container
	SetContainer(c Container) QuerySet
	Filter(c Conditioner) QuerySet
	Get(c Conditioner) (*Instance, error)
	Exists() (bool, error)
	Count() (int64, error)
	Update(values Container) (int64, error)
	Delete() (int64, error)
}

type GenericQuerySet struct {
	model     *Model
	container Container
	database  string
	columns   []string
	cond      Conditioner
}

func (qs GenericQuerySet) dbError(err error) error {
	trace := ErrorTrace{App: qs.model.app, Model: qs.model, Err: err}
	return &DatabaseError{qs.database, trace}
}

func (qs GenericQuerySet) containerError(err error) error {
	trace := ErrorTrace{App: qs.model.app, Model: qs.model, Err: err}
	return &ContainerError{trace}
}

func (qs GenericQuerySet) addConditioner(c Conditioner) GenericQuerySet {
	if qs.cond == nil {
		if cond, ok := c.(Q); ok {
			qs.cond = Filter{root: cond}
		} else {
			qs.cond = c
		}
	} else {
		qs.cond = qs.cond.And(c)
	}
	return qs
}

func (qs GenericQuerySet) Query() (string, []interface{}) {
	driver := ""
	db, ok := databases[qs.database]
	if !ok {
		db, ok = databases["default"]
	}
	if ok {
		driver = db.Driver
	}
	columns := make([]string, 0, len(qs.columns))
	for _, name := range qs.columns {
		col := name
		if field, ok := qs.model.fields[name]; ok {
			col = field.DBColumn(name)
		}
		columns = append(columns, fmt.Sprintf("\"%s\"", col))
	}
	stmt := fmt.Sprintf(
		"SELECT %s FROM %s", strings.Join(columns, ", "), qs.model.Table(),
	)
	if qs.cond != nil {
		pred, values := qs.cond.Predicate(driver, 1)
		stmt += fmt.Sprintf(" WHERE %s", pred)
		return stmt, values
	} else {
		return stmt, make([]interface{}, 0)
	}
}

func (qs GenericQuerySet) Model() *Model {
	return qs.model
}

func (qs GenericQuerySet) Container() Container {
	if b, ok := qs.container.(Builder); ok {
		return b.New()
	} else {
		ct := reflect.TypeOf(qs.container)
		if ct.Kind() == reflect.Ptr {
			ct = ct.Elem()
		}
		return reflect.New(ct).Interface()
	}
}

func (qs GenericQuerySet) SetContainer(container Container) QuerySet {
	if isValidContainer(container) {
		qs.container = container
	} else {
		qs.container = nil
	}
	return qs
}

func (qs GenericQuerySet) Filter(c Conditioner) QuerySet {
	return qs.addConditioner(c)
}

func (qs GenericQuerySet) Load() ([]*Instance, error) {
	result := []*Instance{}
	db, ok := databases[qs.database]
	if !ok {
		return nil, qs.dbError(fmt.Errorf("db not found: %s", qs.database))
	}
	if qs.container == nil {
		return nil, qs.containerError(fmt.Errorf("invalid container"))
	}
	container := qs.Container()
	recipients := getRecipients(container, qs.columns, qs.model)
	if len(recipients) != len(qs.columns) {
		err := fmt.Errorf("invalid container recipients")
		return nil, qs.containerError(err)
	}
	stmt, values := qs.Query()
	rows, err := db.Conn.Query(stmt, values...)
	if err != nil {
		return nil, qs.dbError(err)
	}
	defer rows.Close()
	for rows.Next() {
		container = qs.Container()
		if _, ok := container.(Setter); !ok {
			recipients = getRecipients(container, qs.columns, qs.model)
		}
		err := rows.Scan(recipients...)
		if err != nil {
			return nil, qs.containerError(err)
		}
		instance := &Instance{qs.model, container}
		if _, ok := container.(Setter); ok {
			for i, name := range qs.columns {
				val := reflect.Indirect(
					reflect.ValueOf(recipients[i]),
				).Interface()
				instance.Set(name, val)
			}
		}
		result = append(result, instance)
	}
	err = rows.Err()
	if err != nil {
		return nil, qs.dbError(err)
	}
	return result, nil
}

func (qs GenericQuerySet) Get(c Conditioner) (*Instance, error) {
	qs = qs.addConditioner(c)
	db, ok := databases[qs.database]
	if !ok {
		return nil, qs.dbError(fmt.Errorf("db not found: %s", qs.database))
	}
	container := qs.Container()
	recipients := getRecipients(container, qs.columns, qs.model)
	if len(recipients) != len(qs.columns) {
		err := fmt.Errorf("invalid container recipients")
		return nil, qs.containerError(err)
	}
	stmt, values := qs.Query()
	err := db.Conn.QueryRow(stmt, values...).Scan(recipients...)
	if err != nil {
		return nil, qs.dbError(err)
	}
	instance := &Instance{qs.model, container}
	if _, ok := container.(Setter); ok {
		for i, name := range qs.columns {
			val := reflect.Indirect(
				reflect.ValueOf(recipients[i]),
			).Interface()
			instance.Set(name, val)
		}
	}
	return instance, nil
}

func (qs GenericQuerySet) Exists() (bool, error) {
	db, ok := databases[qs.database]
	if !ok {
		return false, qs.dbError(fmt.Errorf("db not found: %s", qs.database))
	}
	var exists bool
	qs.columns = []string{qs.model.pk}
	stmt, values := qs.Query()
	stmt = fmt.Sprintf("SELECT EXISTS (%s)", stmt)
	err := db.Conn.QueryRow(stmt, values...).Scan(&exists)
	if err != nil {
		return false, qs.dbError(err)
	}
	return exists, nil
}

func (qs GenericQuerySet) Count() (int64, error) {
	var count int64
	var values []interface{}
	db, ok := databases[qs.database]
	if !ok {
		return count, qs.dbError(fmt.Errorf("db not found: %s", qs.database))
	}
	stmt := fmt.Sprintf("SELECT COUNT(*) FROM %s", qs.model.Table())
	if qs.cond != nil {
		pred, vals := qs.cond.Predicate(db.Driver, 1)
		stmt += fmt.Sprintf(" WHERE %s", pred)
		values = vals
	}
	err := db.Conn.QueryRow(stmt, values...).Scan(&count)
	if err != nil {
		return count, qs.dbError(err)
	}
	return count, nil
}

func (qs GenericQuerySet) Update(values Container) (int64, error) {
	db, ok := databases[qs.database]
	if !ok {
		return 0, qs.dbError(fmt.Errorf("db not found: %s", qs.database))
	}
	if !isValidContainer(values) {
		err := fmt.Errorf("invalid values container")
		return 0, qs.containerError(err)
	}
	cols := make([]string, 0, len(qs.model.fields))
	vals := make([]interface{}, 0)
	index := 1
	for name := range qs.model.fields {
		if getter, ok := values.(Getter); ok {
			if val, ok := getter.Get(name); ok {
				cols = append(cols, fmt.Sprintf("\"%s\" = $%d", name, index))
				vals = append(vals, val)
				index += 1
			}
		} else if val, ok := getStructField(values, name); ok {
			cols = append(cols, fmt.Sprintf("\"%s\" = $%d", name, index))
			vals = append(vals, val)
			index += 1
		}
	}
	stmt := fmt.Sprintf(
		"UPDATE \"%s\" SET %s", qs.model.Table(), strings.Join(cols, ", "),
	)
	if qs.cond != nil {
		pred, pVals := qs.cond.Predicate(db.Driver, index)
		stmt += fmt.Sprintf(" WHERE %s", pred)
		vals = append(vals, pVals...)
	}
	result, err := db.Conn.Exec(stmt, vals...)
	if err != nil {
		return 0, qs.dbError(err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return 0, qs.dbError(err)
	}
	return count, nil
}

func (qs GenericQuerySet) Delete() (int64, error) {
	db, ok := databases[qs.database]
	if !ok {
		return 0, qs.dbError(fmt.Errorf("db not found: %s", qs.database))
	}
	stmt := fmt.Sprintf("DELETE FROM %s", qs.model.Table())
	values := make([]interface{}, 0)
	if qs.cond != nil {
		pred, vals := qs.cond.Predicate(db.Driver, 1)
		stmt += fmt.Sprintf(" WHERE %s", pred)
		values = append(values, vals)
	}
	result, err := db.Conn.Exec(stmt, values...)
	if err != nil {
		return 0, qs.dbError(err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return 0, qs.dbError(err)
	}
	return count, nil
}
