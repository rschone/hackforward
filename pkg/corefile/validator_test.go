package corefile

import (
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidator_validateStructure(t *testing.T) {
	type testStruct struct {
		Str string `check:"nonempty"`
		Num int    `check:"lt(5)"`
	}
	type testStructWithEmptyTag struct {
		Str string `check:" "`
	}
	v := &validator{log: &mockLogger{}, checkers: defaultChecks}
	tests := []struct {
		name      string
		structure any
		wantErr   bool
	}{
		{
			name:      "valid structure",
			structure: testStruct{Str: "a", Num: 3},
		},
		{
			name:      "not a structure",
			structure: 5,
			wantErr:   true,
		},
		{
			name:      "first field invalid",
			structure: testStruct{},
			wantErr:   true,
		},
		{
			name:      "other field invalid",
			structure: testStruct{Num: 10},
			wantErr:   true,
		},
		{
			name:      "field definition with empty tag",
			structure: testStructWithEmptyTag{},
			wantErr:   true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			structVal := reflect.ValueOf(tt.structure)
			if err := v.validateStructure(structVal); (err != nil) != tt.wantErr {
				t.Errorf("Validator.validateStructure() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidator_validateStructure_customCheckExecuted(t *testing.T) {
	s := &testStruct{Str: "fail"}
	structVal := reflect.ValueOf(s).Elem()
	v := &validator{log: &mockLogger{}, checkers: defaultChecks}
	err := v.validateStructure(structVal)
	assert.NotNil(t, err)
}

type mockLogger struct{}

func (*mockLogger) Err(msg string) error {
	return errors.New(msg)
}

func (*mockLogger) Errf(format string, args ...interface{}) error {
	return fmt.Errorf(format, args...)
}

func TestValidator_validateField(t *testing.T) {
	v := &validator{log: &mockLogger{}, checkers: defaultChecks}
	tests := []struct {
		name    string
		val     reflect.Value
		tag     string
		wantErr bool
	}{
		{
			name: "valid check with exact number of arguments",
			val:  reflect.ValueOf(3),
			tag:  "lt(5)",
		},
		{
			name: "valid check with arbitrary number of arguments",
			val:  reflect.ValueOf("a"),
			tag:  "oneOf(a|b|c)",
		},
		{
			name:    "unknown checker",
			val:     reflect.ValueOf(3),
			tag:     "uknownChecker",
			wantErr: true,
		},
		{
			name:    "missing arguments",
			val:     reflect.ValueOf(3),
			tag:     "lt()",
			wantErr: true,
		},
		{
			name:    "too many arguments",
			val:     reflect.ValueOf(3),
			tag:     "lt(1|2)",
			wantErr: true,
		},
		{
			name:    "empty tag",
			val:     reflect.ValueOf("a"),
			tag:     "",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := v.validateField(tt.val, tt.tag); (err != nil) != tt.wantErr {
				t.Errorf("Validator.validateField() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

type ValueReceiver struct {
	output error
}
type PointerReceiver struct {
	output error
}

func (v ValueReceiver) Check() error {
	return v.output
}

func (p *PointerReceiver) Check() error {
	return p.output
}

func TestValidator_executeCustomChecks(t *testing.T) {
	log := &mockLogger{}
	v := &validator{log: log}
	wantErr := errors.New("custom condition failed")

	for i, value := range []reflect.Value{
		reflect.ValueOf(ValueReceiver{output: wantErr}),
		reflect.ValueOf(&ValueReceiver{output: wantErr}),
		reflect.ValueOf(&PointerReceiver{output: wantErr}),
	} {
		t.Log("Executing test ", i)
		err := v.executeCustomChecks(value)
		if err == nil || err.Error() != "custom check failed: "+wantErr.Error() {
			t.Error("Unexpected error or value receiver Check method not called:", err)
		}
	}
}

func Test_nonempty(t *testing.T) {
	tests := []struct {
		name    string
		value   reflect.Value
		wantErr bool
	}{
		{
			name:    "empty string",
			value:   reflect.ValueOf(""),
			wantErr: true,
		},
		{
			name:  "non-empty string",
			value: reflect.ValueOf("test string"),
		},
		{
			name:    "nil value",
			value:   reflect.Zero(reflect.TypeOf((*string)(nil)).Elem()),
			wantErr: true,
		},
		{
			name:    "nil struct",
			value:   reflect.Zero(reflect.TypeOf((*struct{})(nil)).Elem()),
			wantErr: true,
		},
		{
			name:    "int value",
			value:   reflect.ValueOf(1),
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := nonempty(tt.value, []string{}, ""); (err != nil) != tt.wantErr {
				t.Errorf("nonempty() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestOneOf(t *testing.T) {
	tests := []struct {
		name  string
		value interface{}
		args  []string
		want  error
	}{
		{
			name:  "string found in args",
			value: "test",
			args:  []string{"test", "foo", "bar"},
		},
		{
			name:  "string not found in args",
			value: "missing",
			args:  []string{"test", "foo", "bar"},
			want:  errors.New("should be one of [test foo bar]"),
		},
		{
			name:  "integer found in args",
			value: 1,
			args:  []string{"1", "2", "3"},
			want:  errors.New("should be one of [1 2 3]"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := oneOf(reflect.ValueOf(tt.value), tt.args, "")
			if !reflect.DeepEqual(err, tt.want) {
				t.Errorf("oneOf() error = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestNumericComp(t *testing.T) {
	tests := []struct {
		name      string
		value     interface{}
		args      []string
		specifier string
		want      error
	}{
		{
			name:      "int <",
			value:     5,
			args:      []string{"10"},
			specifier: "<",
		},
		{
			name:      "int < failed",
			value:     10,
			args:      []string{"5"},
			specifier: "<",
			want:      fmt.Errorf("should be < 5"),
		},
		{
			name:      "int <=",
			value:     5,
			args:      []string{"10"},
			specifier: "<=",
		},
		{
			name:      "int <= failed",
			value:     10,
			args:      []string{"5"},
			specifier: "<=",
			want:      fmt.Errorf("should be <= 5"),
		},
		{
			name:      "int >",
			value:     15,
			args:      []string{"10"},
			specifier: ">",
		},
		{
			name:      "int > failed",
			value:     1,
			args:      []string{"5"},
			specifier: ">",
			want:      fmt.Errorf("should be > 5"),
		},
		{
			name:      "int >=",
			value:     15,
			args:      []string{"10"},
			specifier: ">=",
		},
		{
			name:      "int => failed",
			value:     1,
			args:      []string{"5"},
			specifier: ">=",
			want:      fmt.Errorf("should be >= 5"),
		},
		{
			name:      "uint <",
			value:     uint8(5),
			args:      []string{"10"},
			specifier: "<",
		},
		{
			name:      "uint < failed",
			value:     uint8(10),
			args:      []string{"5"},
			specifier: "<",
			want:      fmt.Errorf("should be < 5"),
		},
		{
			name:      "uint <=",
			value:     uint8(5),
			args:      []string{"10"},
			specifier: "<=",
		},
		{
			name:      "uint <= failed",
			value:     uint8(10),
			args:      []string{"5"},
			specifier: "<=",
			want:      fmt.Errorf("should be <= 5"),
		},
		{
			name:      "uint >",
			value:     uint8(15),
			args:      []string{"10"},
			specifier: ">",
		},
		{
			name:      "uint > failed",
			value:     uint8(1),
			args:      []string{"5"},
			specifier: ">",
			want:      fmt.Errorf("should be > 5"),
		},
		{
			name:      "uint >=",
			value:     uint8(15),
			args:      []string{"10"},
			specifier: ">=",
		},
		{
			name:      "uint => failed",
			value:     uint8(1),
			args:      []string{"5"},
			specifier: ">=",
			want:      fmt.Errorf("should be >= 5"),
		},
		{
			name:      "float <",
			value:     float32(5),
			args:      []string{"10"},
			specifier: "<",
		},
		{
			name:      "float < failed",
			value:     float32(10),
			args:      []string{"5"},
			specifier: "<",
			want:      fmt.Errorf("should be < 5.00"),
		},
		{
			name:      "float <=",
			value:     float32(5),
			args:      []string{"10"},
			specifier: "<=",
		},
		{
			name:      "float <= failed",
			value:     float32(10),
			args:      []string{"5"},
			specifier: "<=",
			want:      fmt.Errorf("should be <= 5.00"),
		},
		{
			name:      "float >",
			value:     float32(15),
			args:      []string{"10"},
			specifier: ">",
		},
		{
			name:      "float > failed",
			value:     float32(1),
			args:      []string{"5"},
			specifier: ">",
			want:      fmt.Errorf("should be > 5.00"),
		},
		{
			name:      "float >=",
			value:     float32(15),
			args:      []string{"10"},
			specifier: ">=",
		},
		{
			name:      "float => failed",
			value:     float32(1),
			args:      []string{"5"},
			specifier: ">=",
			want:      fmt.Errorf("should be >= 5.00"),
		},
		{
			name:      "unsupported type",
			value:     "10",
			args:      []string{"5"},
			specifier: "<=",
			want:      errors.New("unsupported field type: string"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := numericComp(reflect.ValueOf(tt.value), tt.args, tt.specifier)
			if !reflect.DeepEqual(err, tt.want) {
				t.Errorf("numericComp() error = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestConvertToSameType(t *testing.T) {
	tests := []struct {
		name    string
		value   interface{}
		arg     string
		want    reflect.Value
		wantErr bool
	}{
		{
			name:  "int",
			value: 5,
			arg:   "10",
			want:  reflect.ValueOf(10),
		},
		{
			name:  "int8",
			value: int8(5),
			arg:   "10",
			want:  reflect.ValueOf(int8(10)),
		},
		{
			name:  "int16",
			value: int16(5),
			arg:   "10",
			want:  reflect.ValueOf(int16(10)),
		},
		{
			name:  "int32",
			value: int32(5),
			arg:   "10",
			want:  reflect.ValueOf(int32(10)),
		},
		{
			name:  "int64",
			value: int64(5),
			arg:   "10",
			want:  reflect.ValueOf(int64(10)),
		},
		{
			name:  "uint8",
			value: uint8(5),
			arg:   "10",
			want:  reflect.ValueOf(uint8(10)),
		},
		{
			name:  "uint16",
			value: uint16(5),
			arg:   "10",
			want:  reflect.ValueOf(uint16(10)),
		},
		{
			name:  "uint32",
			value: uint32(5),
			arg:   "10",
			want:  reflect.ValueOf(uint32(10)),
		},
		{
			name:  "uint64",
			value: uint64(5),
			arg:   "10",
			want:  reflect.ValueOf(uint64(10)),
		},
		{
			name:  "float32",
			value: float32(5.5),
			arg:   "10.2",
			want:  reflect.ValueOf(float32(10.2)),
		},
		{
			name:  "float64",
			value: 5.5,
			arg:   "10.2",
			want:  reflect.ValueOf(10.2),
		},
		{
			name:    "string -> unsupported type",
			value:   "5",
			arg:     "10",
			wantErr: true,
		},
		{
			name:    "bool -> unsupported type",
			value:   true,
			arg:     "false",
			wantErr: true,
		},
		{
			name:    "int -> conversion failed",
			value:   5,
			arg:     "10.0",
			wantErr: true,
		},
		{
			name:    "uint -> conversion failed",
			value:   uint8(5),
			arg:     "10.0",
			wantErr: true,
		},
		{
			name:    "float -> conversion failed",
			value:   float32(5.0),
			arg:     "non",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := convertToSameType(reflect.ValueOf(tt.value), tt.arg)
			assert.Equal(t, tt.wantErr, err != nil)
			if err == nil && !reflect.DeepEqual(got.Interface(), tt.want.Interface()) {
				t.Errorf("convertToSameType() got = %v, want %v", got.Interface(), tt.want.Interface())
			}
		})
	}
}
