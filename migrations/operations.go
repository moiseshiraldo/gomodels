package migrations

import (
	"encoding/json"
	"fmt"
	"github.com/moiseshiraldo/gomodels"
)

type Operation interface {
	Name() string
	FromJSON(raw []byte) (Operation, error)
	SetState(state *AppState) error
}

type OperationList []Operation

func (opList OperationList) MarshalJSON() ([]byte, error) {
	result := []map[string]Operation{}
	for _, op := range opList {
		m := map[string]Operation{}
		m[op.Name()] = op
		result = append(result, m)
	}
	return json.Marshal(result)
}

func (op *OperationList) UnmarshalJSON(data []byte) error {
	opList := *op
	rawList := []map[string]json.RawMessage{}
	err := json.Unmarshal(data, &rawList)
	if err != nil {
		return err
	}
	for _, rawMap := range rawList {
		for name, rawOp := range rawMap {
			native, ok := AvailableOperations()[name]
			if !ok {
				return fmt.Errorf("invalid operation: %s", name)
			}
			operation, err := native.FromJSON(rawOp)
			if err != nil {
				return err
			}
			opList = append(*op, operation)
		}
	}
	*op = opList
	return nil
}

type CreateModel struct {
	Model  string
	Fields gomodels.Fields
}

func (op CreateModel) Name() string {
	return "CreateModel"
}

func (op CreateModel) FromJSON(raw []byte) (Operation, error) {
	err := json.Unmarshal(raw, &op)
	return op, err
}

func (op CreateModel) SetState(state *AppState) error {
	if _, found := state.Models[op.Model]; found {
		return fmt.Errorf("duplicate model: %s", op.Model)
	}
	state.Models[op.Model] = gomodels.New(op.Model, op.Fields)
	return nil
}

type Field struct {
	Type    string
	Options gomodels.Field
}

func (f *Field) UnmarshalJSON(data []byte) error {
	obj := struct {
		Type    string
		Options json.RawMessage
	}{}
	err := json.Unmarshal(data, &obj)
	if err != nil {
		return err
	}
	f.Type = obj.Type
	native, ok := gomodels.AvailableFields()[obj.Type]
	if !ok {
		return fmt.Errorf("invalid field type: %s", obj.Type)
	}
	f.Options, err = native.FromJSON(obj.Options)
	if err != nil {
		return err
	}
	return nil
}

type DeleteModel struct {
	Model string
}

func (op DeleteModel) Name() string {
	return "DeleteModel"
}

func (op DeleteModel) FromJSON(raw []byte) (Operation, error) {
	err := json.Unmarshal(raw, &op)
	return op, err
}

func (op DeleteModel) SetState(state *AppState) error {
	if _, ok := state.Models[op.Model]; !ok {
		return fmt.Errorf("delete model not found: %s", op.Model)
	}
	delete(state.Models, op.Model)
	return nil
}

type AddFields struct {
	Model  string
	Fields gomodels.Fields
}

func (op AddFields) Name() string {
	return "AddFields"
}

func (op AddFields) FromJSON(raw []byte) (Operation, error) {
	err := json.Unmarshal(raw, &op)
	return op, err
}

func (op AddFields) SetState(state *AppState) error {
	if _, ok := state.Models[op.Model]; !ok {
		return fmt.Errorf("add fields: model not found: %s", op.Model)
	}
	fields := state.Models[op.Model].Fields()
	for name, field := range op.Fields {
		if _, found := fields[name]; found {
			return fmt.Errorf("%s: duplicate field: %s", op.Model, name)
		}
		fields[name] = field
	}
	delete(state.Models, op.Model)
	state.Models[op.Model] = gomodels.New(op.Model, fields)
	return nil
}

type RemoveFields struct {
	Model  string
	Fields []string
}

func (op RemoveFields) Name() string {
	return "RemoveFields"
}

func (op RemoveFields) FromJSON(raw []byte) (Operation, error) {
	err := json.Unmarshal(raw, &op)
	return op, err
}

func (op RemoveFields) SetState(state *AppState) error {
	if _, ok := state.Models[op.Model]; !ok {
		return fmt.Errorf("remove fields: model not found: %s", op.Model)
	}
	fields := state.Models[op.Model].Fields()
	for _, name := range op.Fields {
		if _, ok := fields[name]; !ok {
			return fmt.Errorf("%s: remove field not found: %s", op.Model, name)
		}
		delete(fields, name)
	}
	delete(state.Models, op.Model)
	state.Models[op.Model] = gomodels.New(op.Model, fields)
	return nil
}

func AvailableOperations() map[string]Operation {
	return map[string]Operation{
		"CreateModel": CreateModel{},
		"DeleteModel": DeleteModel{},
		"AddFields":   AddFields{},
	}
}
