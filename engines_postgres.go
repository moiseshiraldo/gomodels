package gomodels

import (
	"database/sql"
	"fmt"
	"strings"
)

type PostgresEngine struct {
	*sql.DB
}

func (e PostgresEngine) Start(db *Database) (Engine, error) {
	credentials := fmt.Sprintf(
		"dbname=%s user=%s password=%s sslmode=disable",
		db.Name, db.User, db.Password,
	)
	conn, err := sql.Open(db.Driver, credentials)
	if err != nil {
		return nil, err
	}
	e.DB = conn
	db.Conn = conn
	return e, nil
}

func (e PostgresEngine) InsertRow(
	model *Model, container Container, fields ...string,
) (int64, error) {
	cols := make([]string, 0, len(model.fields))
	vals := make([]interface{}, 0, len(model.fields))
	placeholders := make([]string, 0, len(model.fields))
	allFields := len(fields) == 0
	index := 1
	for name, field := range model.fields {
		if !field.IsAuto() && (allFields || fieldInList(name, fields)) {
			var value Value
			if getter, ok := container.(Getter); ok {
				if val, ok := getter.Get(name); ok {
					value = val
				}
			} else if val, ok := getStructField(container, name); ok {
				value = val
			}
			if value != nil {
				cols = append(cols, fmt.Sprintf("\"%s\"", field.DBColumn(name)))
				vals = append(vals, value)
				placeholders = append(placeholders, fmt.Sprintf("$%d", index))
				index += 1
			}
		}
	}
	stmt := fmt.Sprintf(
		"INSERT INTO \"%s\" (%s) VALUES (%s) RETURNING \"%s\"",
		model.Table(),
		strings.Join(cols, ", "),
		strings.Join(placeholders, ", "),
		model.pk,
	)
	var pk int64
	err := e.QueryRow(stmt, vals...).Scan(&pk)
	if err != nil {
		return pk, err
	}
	return pk, nil
}

func (e PostgresEngine) UpdateRows(
	model *Model, cont Container, conditioner Conditioner, fields ...string,
) (int64, error) {
	vals := make([]interface{}, 0, len(model.fields))
	cols := make([]string, 0, len(model.fields))
	allFields := len(fields) == 0
	index := 1
	for name, field := range model.fields {
		if name != model.pk && (allFields || fieldInList(name, fields)) {
			var value Value
			if getter, ok := cont.(Getter); ok {
				if val, ok := getter.Get(name); ok {
					value = val
				}
			} else if val, ok := getStructField(cont, name); ok {
				value = val
			}
			if value != nil {
				col := fmt.Sprintf(
					"\"%s\" = $%d", field.DBColumn(name), index,
				)
				cols = append(cols, col)
				vals = append(vals, value)
				index += 1
			}
		}
	}
	stmt := fmt.Sprintf(
		"UPDATE \"%s\" SET %s", model.Table(), strings.Join(cols, ", "),
	)
	if conditioner != nil {
		pred, pVals := conditioner.Predicate("postgres", index)
		stmt += fmt.Sprintf(" WHERE %s", pred)
		vals = append(vals, pVals...)
	}
	result, err := e.Exec(stmt, vals...)
	if err != nil {
		return 0, err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}
	return rows, nil
}
