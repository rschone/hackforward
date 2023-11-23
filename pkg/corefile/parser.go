package corefile

import (
	"reflect"
	"strings"

	"github.com/coredns/caddy"
)

const (
	pluginArgsFieldName = "Arguments"
	cfTag               = "cf"
	defaultTag          = "default"
	checkTag            = "check"
)

// Initializer is implemented by a structure when custom structure initialization is required.
type Initializer interface {
	Init() error
}

type parser struct {
	lexer     *caddy.Controller
	log       logger
	validator validator
}

// Parse parses the input provided by caddy and fills the configuration into provided pointer to a custom structure.
func Parse(c *caddy.Controller, v any) error {
	p := parser{lexer: c, log: c, validator: validator{log: c, checkers: defaultChecks}}
	return p.parse(v)
}

func (p *parser) parse(s any) error {
	if !isPointerToStruct(s) {
		return p.log.Err("invalid argument: pointer to a structure expected")
	}
	if !p.lexer.Next() {
		return p.log.Err("plugin name expected")
	}

	pluginName := p.lexer.Val()
	structVal := reflect.ValueOf(s).Elem()

	if err := p.parsePluginHeader(structVal, pluginName); err != nil {
		return err
	}

	if err := p.applyDefaults(structVal); err != nil {
		return err
	}

	if p.lexer.Next() {
		if p.lexer.Val() != "{" {
			return p.log.Err("'{' expected")
		}
		return p.parseStructure(structVal, pluginName)
	}

	return p.applyDefaults(structVal)
}

func (p *parser) parsePluginHeader(structVal reflect.Value, pluginName string) error {
	pluginArgs := p.lexer.RemainingArgs()
	if len(pluginArgs) > 0 {
		if err := assignToField(structVal, pluginArgsFieldName, pluginArgs); err != nil {
			return p.log.Errf("cannot store plugin '%s' arguments into field '%s': %v", pluginName, pluginArgsFieldName, err)
		}
	}
	return nil
}

func (p *parser) parseStructure(structVal reflect.Value, structName string) error {
	for p.lexer.Next() {
		if p.lexer.Val() == "}" {
			return p.validator.validateStructure(structVal)
		}

		property := p.lexer.Val()
		propValues := p.lexer.RemainingArgs()

		if len(propValues) == 0 {
			field := findFieldByTag(structVal, property)
			if !field.IsValid() {
				return p.log.Errf("property '%s' in structure '%s' not found", property, structName)
			}

			if field.Type().Kind() == reflect.Pointer {
				if field.IsNil() {
					newMem := reflect.New(field.Type().Elem())
					field.Set(newMem)
				}
				field = field.Elem()
				if err := p.applyDefaults(field); err != nil {
					return err
				}
			}
			if field.Type().Kind() == reflect.Struct {
				if !p.lexer.Next() || p.lexer.Val() != "{" {
					return p.log.Errf("structure opening character '{' expected, got '%s'", p.lexer.Val())
				}
				if err := p.parseStructure(field, property); err != nil {
					return err
				}
			}
			// or it is a field without value that keeps its default value that has been set by applying defaults
			// or zero value if a default value had not been present
		} else {
			field := findFieldByTag(structVal, property)
			if !field.IsValid() {
				return p.log.Err("field not found: " + property)
			}

			value := strings.Join(propValues, ",")
			if err := assignFromString(field, value); err != nil {
				return p.log.Errf("assigning property value failed: %v", err)
			}
		}
	}

	return p.log.Err("'}' expected")
}

func (p *parser) applyDefaults(structVal reflect.Value) error {
	structType := structVal.Type()
	for i := 0; i < structVal.NumField(); i++ {
		fieldType := structType.Field(i)
		field := structVal.Field(i)
		if field.Kind() == reflect.Struct {
			if err := p.applyDefaults(field); err != nil {
				return err
			}
		} else {
			if defaultValue, ok := fieldType.Tag.Lookup(defaultTag); ok {
				if err := assignFromString(field, defaultValue); err != nil {
					return p.log.Errf("apply defaults to property: %v", err)
				}
			}
		}
	}
	return p.executeCustomInit(structVal)
}

func (p *parser) executeCustomInit(structVal reflect.Value) error {
	if itf, ok := structVal.Addr().Interface().(Initializer); ok && itf != nil {
		if err := itf.Init(); err != nil {
			return p.log.Errf("custom init failed: %v", err)
		}
	}
	return nil
}
