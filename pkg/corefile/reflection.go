package corefile

import (
	"errors"
	"fmt"
	"net"
	"reflect"
	"strconv"
	"strings"
	"time"
)

var (
	errNotSettable  = errors.New("target is not settable")
	errTypeMishmash = errors.New("target is not assignable due the type mishmash")
)

func isPointerToStruct(ps interface{}) bool {
	t := reflect.TypeOf(ps)
	if t.Kind() != reflect.Pointer {
		return false
	}

	s := t.Elem()
	return s != nil && s.Kind() == reflect.Struct
}

func assignToField(structVal reflect.Value, fieldName string, data interface{}) error {
	field := structVal.FieldByName(fieldName)
	if !field.IsValid() {
		return fmt.Errorf("field '%s' not found", fieldName)
	}
	return assignTo(field, data)
}

func assignTo(target reflect.Value, data interface{}) error {
	if err := checkIfAssignable(target, data); err != nil {
		return err
	}
	dataVal := reflect.ValueOf(data)
	target.Set(dataVal)
	return nil
}

func checkIfAssignable(target reflect.Value, data interface{}) error {
	if !target.CanSet() {
		return errNotSettable
	}

	dataType := reflect.TypeOf(data)
	valueType := target.Type()
	if !dataType.AssignableTo(valueType) {
		return errTypeMishmash
	}

	return nil
}

func findFieldByTag(structVal reflect.Value, name string) reflect.Value {
	structType := structVal.Type()
	for i := 0; i < structType.NumField(); i++ {
		field := structType.Field(i)
		if tag, ok := field.Tag.Lookup(cfTag); ok {
			if ok && tag == name {
				return structVal.Field(i)
			}
		}
	}
	return reflect.Value{}
}

func assignFromString(target reflect.Value, input string) error {
	switch target.Kind() {
	case reflect.String:
		target.SetString(input)
	case reflect.Int:
		fallthrough
	case reflect.Int8:
		fallthrough
	case reflect.Int16:
		fallthrough
	case reflect.Int32:
		fallthrough
	case reflect.Int64:
		if target.Type().Name() == "Duration" {
			durationValue, err := time.ParseDuration(input)
			if err != nil {
				return err
			}
			target.SetInt(int64(durationValue))
		} else {
			intValue, err := strconv.Atoi(input)
			if err != nil {
				return err
			}
			target.SetInt(int64(intValue))
		}
	case reflect.Float32:
		floatValue, err := strconv.ParseFloat(input, 32)
		if err != nil {
			return err
		}
		target.SetFloat(floatValue)
	case reflect.Float64:
		floatValue, err := strconv.ParseFloat(input, 64)
		if err != nil {
			return err
		}
		target.SetFloat(floatValue)
	case reflect.Bool:
		boolValue, err := strconv.ParseBool(input)
		if err != nil {
			return err
		}
		target.SetBool(boolValue)
	case reflect.Slice:
		switch target.Type().Elem().Kind() {
		case reflect.String:
			stringSlice := strings.Split(input, ",")
			target.Set(reflect.ValueOf(stringSlice))
		case reflect.Int:
			var intSlice []int
			for _, str := range strings.Split(input, ",") {
				intValue, err := strconv.Atoi(str)
				if err != nil {
					return err
				}
				intSlice = append(intSlice, intValue)
			}
			target.Set(reflect.ValueOf(intSlice))
		case reflect.Uint8:
			ip := net.ParseIP(input)
			if ip == nil {
				return fmt.Errorf("invalid IP: %s", input)
			}
			target.Set(reflect.ValueOf(ip))
		default:
			return fmt.Errorf("unsupported slice type: %v", target.Type())
		}
	default:
		return fmt.Errorf("unsupported type: %v", target.Type())
	}
	return nil
}
