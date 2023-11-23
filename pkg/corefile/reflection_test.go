package corefile

import (
	"net"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func Test_isPointerToStruct(t *testing.T) {
	tests := []struct {
		name  string
		input interface{}
		want  bool
	}{
		{
			name:  "valid pointer to struct",
			input: &testStruct{},
			want:  true,
		},
		{
			name:  "pointer to slice",
			input: &[]int{1},
		},
		{
			name:  "struct",
			input: testStruct{},
		},
		{
			name:  "int",
			input: 5,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, isPointerToStruct(tt.input))
		})
	}
}

func Test_checkIfAssignable(t *testing.T) {
	var number int
	ts := testStruct{}
	ps := &testStruct{}

	tests := []struct {
		name   string
		target reflect.Value
		data   interface{}
		want   error
	}{
		{
			name:   "simple assignable var",
			target: reflect.ValueOf(number),
			data:   1,
			want:   errNotSettable,
		},
		{
			name:   "pointer to assignable var",
			target: reflect.ValueOf(&number),
			data:   1,
			want:   errNotSettable,
		},
		{
			name:   "exported field of struct pointer",
			target: reflect.ValueOf(ps).Elem().FieldByName("IntNum"),
			data:   1,
			want:   nil,
		},
		{
			name:   "exported field of struct pointer",
			target: reflect.ValueOf(ps).Elem().FieldByName("IntNum"),
			data:   "string",
			want:   errTypeMishmash,
		},
		{
			name:   "unexported field of struct pointer",
			target: reflect.ValueOf(ps).Elem().FieldByName("unexported"),
			data:   1,
			want:   errNotSettable,
		},
		{
			name:   "exported field of struct",
			target: reflect.ValueOf(ts).FieldByName("IntNum"),
			data:   1,
			want:   errNotSettable,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := checkIfAssignable(tt.target, tt.data)
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_findFieldByTag(t *testing.T) {
	var s testStruct
	structVal := reflect.ValueOf(s)
	tests := []struct {
		name      string
		structVal reflect.Value
		tagName   string
		want      reflect.Value
	}{
		{
			name:      "find exported field",
			structVal: structVal,
			tagName:   "intnum",
			want:      structVal.FieldByName("IntNum"),
		},
		{
			name:      "find unexported field",
			structVal: structVal,
			tagName:   "unexported",
			want:      structVal.FieldByName("unexported"),
		},
		{
			name:      "find not tagged field",
			structVal: structVal,
			tagName:   "notTagged",
			want:      reflect.Value{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			field := findFieldByTag(tt.structVal, tt.tagName)
			assert.Equal(t, tt.want, field)
		})
	}
}

func Test_assignTo(t *testing.T) {
	ps := &testStruct{}

	exportedField := reflect.ValueOf(ps).Elem().FieldByName("IntNum")
	err := assignTo(exportedField, 5)
	assert.Nil(t, err)
	assert.Equal(t, ps.IntNum, 5)

	unexportedField := reflect.ValueOf(ps).Elem().FieldByName("unexported")
	err = assignTo(unexportedField, 5)
	assert.Equal(t, err, errNotSettable)
}

func Test_assignToField1(t *testing.T) {
	ps := &testStruct{}
	structVal := reflect.ValueOf(ps).Elem()

	err := assignToField(structVal, "IntNum", 5)
	assert.Nil(t, err)
	assert.Equal(t, ps.IntNum, 5)

	err = assignToField(structVal, "unexported", 5)
	assert.Equal(t, err, errNotSettable)

	err = assignToField(structVal, "notExisting", 5)
	assert.NotNil(t, err)
}

func Test_assignFromString(t *testing.T) {
	p := &testStruct{}
	tests := []struct {
		field   string
		input   string
		wantErr bool
		want    interface{}
	}{
		{field: "Str", input: "sth", want: "sth"},
		{field: "IntNum", input: "1", want: 1},
		{field: "Int8Num", input: "1", want: int8(1)},
		{field: "Int16Num", input: "1", want: int16(1)},
		{field: "Int32Num", input: "1", want: int32(1)},
		{field: "Int64Num", input: "1", want: int64(1)},
		{field: "Duration", input: "10s", want: 10 * time.Second},
		{field: "Real32", input: "1.0", want: float32(1)},
		{field: "Real64", input: "1.0", want: float64(1)},
		{field: "Boolean", input: "true", want: true},
		{field: "StrSlice", input: "a,b,c", want: []string{"a", "b", "c"}},
		{field: "IntSlice", input: "1,2,3", want: []int{1, 2, 3}},
		{field: "IP", input: "1.2.3.4", want: net.ParseIP("1.2.3.4")},
		{field: "Int64Num", input: "ff", wantErr: true},
		{field: "Duration", input: "x", wantErr: true},
		{field: "Real32", input: "x0", wantErr: true},
		{field: "Real64", input: "x0", wantErr: true},
		{field: "Boolean", input: "y", wantErr: true},
		{field: "IntSlice", input: "a", wantErr: true},
		{field: "IP", input: "1.2.3.", wantErr: true},
		{field: "Unsupported", input: "{}", wantErr: true},
		{field: "UnsupportedSlice", input: "{},{}", wantErr: true},
	}
	for _, tt := range tests {
		t.Run("check assign to field "+tt.field, func(t *testing.T) {
			target := reflect.ValueOf(p).Elem().FieldByName(tt.field)
			err := assignFromString(target, tt.input)
			assert.Equal(t, tt.wantErr, err != nil)
			if !tt.wantErr {
				assert.Equal(t, tt.want, target.Interface())
			}
		})
	}
}
