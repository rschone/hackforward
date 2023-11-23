package corefile

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"k8s.io/utils/strings/slices"
)

type validator struct {
	log      logger
	checkers map[string]checker
}

type checkFunc func(val reflect.Value, args []string, specifier string) error

type checker struct {
	checkFunc checkFunc
	specifier string
}

var defaultChecks = map[string]checker{
	"nonempty": {checkFunc: nonempty},
	"oneof":    {checkFunc: oneOf},
	"lt":       {checkFunc: numericComp, specifier: "<"},
	"lte":      {checkFunc: numericComp, specifier: "<="},
	"gt":       {checkFunc: numericComp, specifier: ">"},
	"gte":      {checkFunc: numericComp, specifier: ">="},
}

// CustomChecker represents a custom validation function call.
type CustomChecker interface {
	Check() error
}

func (v *validator) validateStructure(structVal reflect.Value) error {
	if structVal.Kind() != reflect.Struct {
		return v.log.Err("not a struct")
	}

	for i := 0; i < structVal.NumField(); i++ {
		field := structVal.Type().Field(i)
		if tags, ok := field.Tag.Lookup(checkTag); ok && len(tags) > 0 {
			fieldVal := structVal.Field(i)
			for _, tag := range strings.Split(tags, ",") {
				tag = strings.TrimSpace(tag)
				if len(tag) == 0 {
					return v.log.Errf("empty '%s' tag not allowed", checkTag)
				}

				if err := v.validateField(fieldVal, tags); err != nil {
					return v.log.Errf("%s: %w", field.Name, err)
				}
			}
		}
	}

	return v.executeCustomChecks(structVal)
}

func (v *validator) validateField(val reflect.Value, tag string) error {
	for _, condition := range strings.Split(tag, ",") {
		condition = strings.TrimSpace(condition)
		var checkerName string
		var args []string
		if strings.Contains(condition, "(") {
			start := strings.Index(condition, "(")
			end := strings.Index(condition, ")")
			checkerName = condition[:start]
			content := condition[start+1 : end]
			args = strings.Split(content, "|")
		} else {
			checkerName = condition
		}

		checker, ok := v.checkers[strings.ToLower(checkerName)]
		if !ok {
			return errors.New("unknown checker")
		}

		if err := checker.checkFunc(val, args, checker.specifier); err != nil {
			return fmt.Errorf("%s: %w", checkerName, err)
		}
	}
	return nil
}

func (v *validator) executeCustomChecks(structVal reflect.Value) error {
	if structVal.CanAddr() {
		if itf, ok := structVal.Addr().Interface().(CustomChecker); ok && itf != nil {
			if err := itf.Check(); err != nil {
				return v.log.Errf("custom check failed: %v", err)
			}
		}
	}

	if itf, ok := structVal.Interface().(CustomChecker); ok && itf != nil {
		if err := itf.Check(); err != nil {
			return v.log.Errf("custom check failed: %w", err)
		}
		return nil
	}

	return nil
}

func nonempty(v reflect.Value, args []string, _ string) error {
	if len(args) != 0 {
		return fmt.Errorf("nonempty expects no arguments")
	}
	zero := reflect.Zero(v.Type()).Interface()
	if reflect.DeepEqual(v.Interface(), zero) {
		return errors.New("cannot be empty")
	}
	return nil
}

func oneOf(v reflect.Value, args []string, _ string) error {
	if len(args) == 0 {
		return fmt.Errorf("oneOf expects at least one argument")
	}
	if !slices.Contains(args, v.String()) {
		return fmt.Errorf("should be one of %v", args)
	}
	return nil
}

func numericComp(v reflect.Value, args []string, specifier string) error {
	if len(args) != 1 {
		return fmt.Errorf("comparision expects one argument")
	}
	v2, err := convertToSameType(v, args[0])
	if err != nil {
		return err
	}

	switch v.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		v1Int, v2Int := v.Int(), v2.Int()

		if (specifier == "<" && v1Int >= v2Int) ||
			(specifier == "<=" && v1Int > v2Int) ||
			(specifier == ">" && v1Int <= v2Int) ||
			(specifier == ">=" && v1Int < v2Int) {
			return fmt.Errorf("should be %s %d", specifier, v2Int)
		}

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		v1Uint, v2Uint := v.Uint(), v2.Uint()

		if (specifier == "<" && v1Uint >= v2Uint) ||
			(specifier == "<=" && v1Uint > v2Uint) ||
			(specifier == ">" && v1Uint <= v2Uint) ||
			(specifier == ">=" && v1Uint < v2Uint) {
			return fmt.Errorf("should be %s %d", specifier, v2Uint)
		}

	case reflect.Float32, reflect.Float64:
		v1Float, v2Float := v.Float(), v2.Float()

		if (specifier == "<" && v1Float >= v2Float) ||
			(specifier == "<=" && v1Float > v2Float) ||
			(specifier == ">" && v1Float <= v2Float) ||
			(specifier == ">=" && v1Float < v2Float) {
			return fmt.Errorf("should be %s %.2f", specifier, v2Float)
		}

	default:
		// unreachable - would already fail in convertToSameType
		return fmt.Errorf("unsupported field type: %v", v.Type())
	}

	return nil
}

func convertToSameType(val reflect.Value, arg string) (reflect.Value, error) {
	argValue := reflect.New(val.Type()).Elem()
	switch val.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		i, err := strconv.ParseInt(arg, 10, 64)
		if err != nil {
			return reflect.Value{}, fmt.Errorf("cannot convert %s to int", arg)
		}
		argValue.SetInt(i)

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		u, err := strconv.ParseUint(arg, 10, 64)
		if err != nil {
			return reflect.Value{}, fmt.Errorf("cannot convert %s to uint", arg)
		}
		argValue.SetUint(u)

	case reflect.Float32, reflect.Float64:
		f, err := strconv.ParseFloat(arg, 64)
		if err != nil {
			return reflect.Value{}, fmt.Errorf("cannot convert %s to float", arg)
		}
		argValue.SetFloat(f)

	default:
		return reflect.Value{}, fmt.Errorf("unsupported field type: %v", val.Type())
	}

	return argValue, nil
}
