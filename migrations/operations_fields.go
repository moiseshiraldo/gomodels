package migrations

import (
	"fmt"
	"github.com/moiseshiraldo/gomodels"
)

type AddFields struct {
	Model  string
	Fields gomodels.Fields
}

func (op AddFields) OpName() string {
	return "AddFields"
}

func (op *AddFields) SetState(state *AppState) error {
	if _, ok := state.Models[op.Model]; !ok {
		return fmt.Errorf("model not found: %s", op.Model)
	}
	model := state.Models[op.Model]
	fields := model.Fields()
	for name, field := range op.Fields {
		if _, found := fields[name]; found {
			return fmt.Errorf("%s: duplicate field: %s", op.Model, name)
		}
		fields[name] = field
	}
	options := gomodels.Options{
		Table: model.Table(), Indexes: model.Indexes(),
	}
	delete(state.Models, op.Model)
	state.Models[op.Model] = gomodels.New(
		op.Model, fields, options,
	).Model
	return nil
}

func (op AddFields) Run(
	tx *gomodels.Transaction,
	state *AppState,
	prevState *AppState,
) error {
	return tx.AddColumns(state.Models[op.Model], op.Fields)
}

func (op AddFields) Backwards(
	tx *gomodels.Transaction,
	state *AppState,
	prevState *AppState,
) error {
	fields := make([]string, 0, len(op.Fields))
	for name := range op.Fields {
		fields = append(fields, name)
	}
	return tx.DropColumns(
		state.Models[op.Model], prevState.Models[op.Model], fields...,
	)
}

type RemoveFields struct {
	Model  string
	Fields []string
}

func (op RemoveFields) OpName() string {
	return "RemoveFields"
}

func (op *RemoveFields) SetState(state *AppState) error {
	if _, ok := state.Models[op.Model]; !ok {
		return fmt.Errorf("model not found: %s", op.Model)
	}
	model := state.Models[op.Model]
	fields := model.Fields()
	for _, name := range op.Fields {
		if _, ok := fields[name]; !ok {
			return fmt.Errorf("%s: field not found: %s", op.Model, name)
		}
		delete(fields, name)
	}
	options := gomodels.Options{
		Table: model.Table(), Indexes: state.Models[op.Model].Indexes(),
	}
	delete(state.Models, op.Model)
	state.Models[op.Model] = gomodels.New(
		op.Model, fields, options,
	).Model
	return nil
}

func (op RemoveFields) Run(
	tx *gomodels.Transaction,
	state *AppState,
	prevState *AppState,
) error {
	return tx.DropColumns(
		prevState.Models[op.Model], state.Models[op.Model], op.Fields...,
	)
}

func (op RemoveFields) Backwards(
	tx *gomodels.Transaction,
	state *AppState,
	prevState *AppState,
) error {
	fields := prevState.Models[op.Model].Fields()
	newFields := gomodels.Fields{}
	for _, name := range op.Fields {
		newFields[name] = fields[name]
	}
	return tx.AddColumns(state.Models[op.Model], newFields)
}
