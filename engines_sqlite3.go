package gomodel

import (
	"fmt"
	"strings"
)

// sqliteOperators holds the supported comparison operators for sqlite.
var sqliteOperators = map[string]string{
	"=":  "=",
	">":  ">",
	">=": ">=",
	"<":  "<",
	"<=": "<=",
}

// SqliteEngine implements the Engine interface for the sqlite3 driver.
type SqliteEngine struct {
	baseSQLEngine
}

// Start implemetns the Start method of the Engine interface.
func (e SqliteEngine) Start(db Database) (Engine, error) {
	conn, err := openDB(db.Driver, db.Name)
	if err != nil {
		return nil, err
	}
	e.baseSQLEngine = baseSQLEngine{
		db:          conn,
		driver:      "sqlite3",
		escapeChar:  "\"",
		pHolderChar: "?",
		operators:   sqliteOperators,
	}
	return e, nil
}

// BeginTx implemetns the BeginTx method of the Engine interface.
func (e SqliteEngine) BeginTx() (Engine, error) {
	tx, err := e.db.Begin()
	if err != nil {
		return nil, err
	}
	e.tx = tx
	return e, nil
}

// copyTable copies the model table to a new one with the given name and
// columns.
func (e SqliteEngine) copyTable(m *Model, name string, fields ...string) error {
	modelCopy := &Model{fields: Fields{}, meta: Options{Table: name}}
	if len(fields) > 0 {
		for _, name := range fields {
			modelCopy.fields[name] = m.fields[name]
		}
	} else {
		for name, field := range m.fields {
			modelCopy.fields[name] = field
		}
	}
	if err := e.CreateTable(modelCopy, true); err != nil {
		return err
	}
	columns := make([]string, 0, len(fields))
	for name, field := range modelCopy.fields {
		columns = append(columns, e.escape(field.DBColumn(name)))
	}
	stmt := fmt.Sprintf(
		"INSERT INTO %s SELECT %s FROM %s",
		e.escape(name), strings.Join(columns, ", "), e.escape(m.Table()),
	)
	_, err := e.executor().Exec(stmt)
	return err
}

// AddColumns implements the AddColumns method of the Engine interface.
func (e SqliteEngine) AddColumns(model *Model, fields Fields) error {
	notNullFields := make([]string, 0, len(fields))
	for name, field := range fields {
		if !field.IsNull() {
			notNullFields = append(notNullFields, name)
		}
		stmt := fmt.Sprintf(
			"ALTER TABLE %s ADD COLUMN %s %s %s",
			e.escape(model.Table()),
			e.escape(field.DBColumn(name)),
			field.DataType("sqlite3"),
			e.sqlColumnOptions(field, true),
		)
		if _, err := e.executor().Exec(stmt); err != nil {
			return err
		}
	}
	if len(notNullFields) > 0 {
		values := Values{}
		for _, name := range notNullFields {
			field := fields[name]
			val, ok := field.DefaultValue()
			if !ok {
				return fmt.Errorf(
					"%s: cannot add not null column without default", name,
				)
			}
			values[name] = val
		}
		if _, err := e.UpdateRows(model, values, QueryOptions{}); err != nil {
			return err
		}
		copyName := fmt.Sprintf("%s__new", model.Table())
		if err := e.copyTable(model, copyName); err != nil {
			return err
		}
		if err := e.DropTable(model); err != nil {
			return err
		}
		copyModel := &Model{meta: Options{Table: copyName}}
		if err := e.RenameTable(copyModel, model); err != nil {
			return err
		}
		for idxName, fields := range model.Indexes() {
			if err := e.AddIndex(model, idxName, fields...); err != nil {
				return err
			}
		}
	}
	return nil
}

// DropColumns implements the DropColumns method of the Engine interface.
//
// Since sqlite3 doesn't support dropping columns, it will perform the operation
// by creating a new table.
func (e SqliteEngine) DropColumns(model *Model, fields ...string) error {
	oldFields := model.Fields()
	keepCols := make([]string, 0, len(oldFields)-len(fields))
	for _, name := range fields {
		delete(oldFields, name)
	}
	for name, field := range oldFields {
		keepCols = append(keepCols, field.DBColumn(name))
	}
	copyName := fmt.Sprintf("%s__new", model.Table())
	if err := e.copyTable(model, copyName, keepCols...); err != nil {
		return err
	}
	if err := e.DropTable(model); err != nil {
		return err
	}
	copyModel := &Model{meta: Options{Table: copyName}}
	if err := e.RenameTable(copyModel, model); err != nil {
		return err
	}
	for idxName, fields := range model.Indexes() {
		if err := e.AddIndex(model, idxName, fields...); err != nil {
			return err
		}
	}
	return nil
}

// GetRows implements the GetRows method of the Engine interface.
func (e SqliteEngine) GetRows(model *Model, opt QueryOptions) (Rows, error) {
	query, err := e.SelectQuery(model, opt)
	if err != nil {
		return nil, err
	}
	if opt.End > 0 {
		query.Stmt = fmt.Sprintf("%s LIMIT %d", query.Stmt, opt.End-opt.Start)
	} else if opt.Start > 0 {
		query.Stmt += " LIMIT -1"
	}
	if opt.Start > 0 {
		query.Stmt = fmt.Sprintf("%s OFFSET %d", query.Stmt, opt.Start)
	}
	return e.executor().Query(query.Stmt, query.Args...)
}
