package migration

import (
	"encoding/json"
	"fmt"
	"github.com/moiseshiraldo/gomodel"
	"testing"
)

type mockedOperation struct {
	name     string
	run      bool
	back     bool
	state    bool
	StateErr bool
	RunErr   bool
}

func (op *mockedOperation) reset() {
	op.run = false
	op.back = false
	op.state = false
	op.StateErr = false
	op.RunErr = false
}

func (op mockedOperation) OpName() string {
	if op.name != "" {
		return op.name
	}
	return "MockedOperation"
}

func (op *mockedOperation) SetState(state *AppState) error {
	if op.StateErr {
		return fmt.Errorf("state error")
	}
	op.state = true
	return nil
}

func (op *mockedOperation) Run(
	engine gomodel.Engine,
	state *AppState,
	prevState *AppState,
) error {
	if op.RunErr {
		return fmt.Errorf("run error")
	}
	op.run = true
	return nil
}

func (op *mockedOperation) Backwards(
	engine gomodel.Engine,
	state *AppState,
	prevState *AppState,
) error {
	if op.RunErr {
		return fmt.Errorf("run error")
	}
	op.back = true
	return nil
}

// TestOperationList tests OperationList marshal/unmarshall methods
func TestOperationList(t *testing.T) {
	// Registers mocked operation
	if _, ok := operationsRegistry["MockedOperation"]; !ok {
		operationsRegistry["MockedOperation"] = &mockedOperation{}
	}

	t.Run("UnmarshalInvalidJSON", func(t *testing.T) {
		opList := OperationList{}
		err := opList.UnmarshalJSON([]byte("-"))
		if _, ok := err.(*json.SyntaxError); !ok {
			t.Errorf("expected json.SyntaxError, got %T", err)
		}
	})

	t.Run("UnmarshalUnknownOperation", func(t *testing.T) {
		opList := OperationList{}
		data := []byte(`[{"UnknownOperation": {}}]`)
		err := opList.UnmarshalJSON(data)
		if err == nil || err.Error() != "invalid operation: UnknownOperation" {
			t.Errorf("expected invalid operation error, got %s", err)
		}
	})

	t.Run("UnmarshalInvalidOperation", func(t *testing.T) {
		opList := OperationList{}
		data := []byte(`[{"MockedOperation": {"StateErr": []}}]`)
		err := opList.UnmarshalJSON(data)
		if _, ok := err.(*json.UnmarshalTypeError); !ok {
			t.Errorf("expected json.UnmarshalTypeError, got %T", err)
		}
	})

	t.Run("UnmarshalValidOperation", func(t *testing.T) {
		opList := OperationList{}
		data := []byte(`[{"MockedOperation": {}}]`)
		err := opList.UnmarshalJSON(data)
		if err != nil {
			t.Fatal(err)
		}
		if len(opList) != 1 {
			t.Error("expected operation list to contain one operation")
		}
	})

	t.Run("Marshal", func(t *testing.T) {
		opList := OperationList{&mockedOperation{}}
		data, err := opList.MarshalJSON()
		if err != nil {
			t.Fatal(err)
		}
		expected := `[{"MockedOperation":{"StateErr":false,"RunErr":false}}]`
		if string(data) != expected {
			t.Fatalf("expected %s, got %s", expected, string(data))
		}
	})
}

// TestRegisterOperation tests the RegisterOperation function
func TestRegisterOperation(t *testing.T) {

	t.Run("Duplicate", func(t *testing.T) {
		op := &mockedOperation{name: "CreateModel"}
		err := RegisterOperation(op)
		expected := "migrations: duplicate operation: CreateModel"
		if err == nil || err.Error() != expected {
			t.Errorf("expected '%s', got '%s'", expected, err)
		}
	})

	t.Run("Success", func(t *testing.T) {
		op := &mockedOperation{name: "CustomOperation"}
		err := RegisterOperation(op)
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := operationsRegistry["CustomOperation"]; !ok {
			t.Error("operation was not registered")
		}
	})
}
