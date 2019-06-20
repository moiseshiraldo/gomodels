package gomodels

import (
	"fmt"
)

type Database struct {
	Engine
	Driver   string
	Name     string
	User     string
	Password string
	name     string
}

func (db Database) BeginTx() (*Transaction, error) {
	engine, err := db.Engine.BeginTx()
	if err != nil {
		return nil, &DatabaseError{db.name, ErrorTrace{Err: err}}
	}
	return &Transaction{engine, db}, nil
}

type Transaction struct {
	Engine
	DB Database
}

type DBSettings map[string]Database

var databases = DBSettings{}

func Databases() DBSettings {
	dbs := DBSettings{}
	for name, db := range databases {
		dbs[name] = db
	}
	return dbs
}

func Start(options DBSettings) error {
	for name, db := range options {
		engine, ok := engines[db.Driver]
		if !ok {
			err := fmt.Errorf("unsupported driver: %s", db.Driver)
			return &DatabaseError{name, ErrorTrace{Err: err}}
		}
		eng, err := engine.Start(&db)
		if err != nil {
			return &DatabaseError{name, ErrorTrace{Err: err}}
		}
		db.Engine = eng
		db.name = name
		db.Password = ""
		databases[name] = db
	}
	if _, ok := databases["default"]; !ok {
		err := fmt.Errorf("missing default database")
		return &DatabaseError{"default", ErrorTrace{Err: err}}
	}
	return nil
}

func Stop() error {
	var err error
	for name, db := range databases {
		if dbErr := db.Stop(); dbErr != nil {
			err = &DatabaseError{name, ErrorTrace{Err: dbErr}}
		}
	}
	return err
}
