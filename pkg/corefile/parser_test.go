package corefile

import (
	"net"
	"testing"
	"time"

	"github.com/coredns/caddy"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
)

//nolint:unused
type testStruct struct {
	Arguments []string
	Str       string        `cf:"str" check:"oneOF(string|initialized)"`
	IntNum    int           `cf:"intnum"`
	Int8Num   int8          `cf:"int8num"`
	Int16Num  int16         `cf:"int16num" default:"99" check:"nonempty,LTE(99)"`
	Int32Num  int32         `cf:"int32num"`
	Int64Num  int64         `cf:"int64num"`
	Duration  time.Duration `cf:"duration"`
	Real32    float32       `cf:"real32"`
	Real64    float64       `cf:"real64"`
	Boolean   bool          `cf:"boolean"`
	StrSlice  []string      `cf:"strslice"`
	IntSlice  []int         `cf:"intslice"`
	IP        net.IP        `cf:"ip"`

	Unsupported      struct{}
	UnsupportedSlice []struct{}
	unexported       int `cf:"unexported"`
	notTagged        int
}

func (t *testStruct) Check() error {
	if t.Str == "fail" {
		return errors.New("fail")
	}
	return nil
}

func (t *testStruct) Init() error {
	t.Str = "initialized"
	return nil
}

func Test_ParseWithCaddy(t *testing.T) {
	tests := []struct {
		name    string
		cfg     string
		want    interface{}
		wantErr bool
	}{
		{
			name: "plugin without body",
			cfg:  "plugin",
			want: testStruct{Str: "initialized", Int16Num: 99},
		},
		{
			name: "plugin without body with arguments",
			cfg:  "plugin arg1 arg2",
			want: testStruct{Arguments: []string{"arg1", "arg2"}, Str: "initialized", Int16Num: 99},
		},
		{
			name: "plugin with arguments, custom init, applying defaults",
			cfg: `plugin arg1 arg2 {
					}`,
			want: testStruct{Arguments: []string{"arg1", "arg2"}, Str: "initialized", Int16Num: 99},
		},
		{
			name: "plugin with properties, overwriting defaults",
			cfg: `plugin {
						int16num 1
						str string
					}`,
			want: testStruct{Str: "string", Int16Num: 1},
		},
		{
			name: "property without value - defaults are applied",
			cfg: `plugin {
						int16num
						str
					}`,
			want: testStruct{Str: "initialized", Int16Num: 99},
		},
		{
			name: "property without value and no defaults, zero value is set",
			cfg: `plugin {
						int8num
					}`,
			want: testStruct{Str: "initialized", Int16Num: 99},
		},
		{
			name:    "missing plugin name",
			cfg:     "",
			wantErr: true,
		},
		{
			name: "unknown property",
			cfg: `plugin {
						unknownTag 1
					}`,
			wantErr: true,
		},
		{
			name: "unknown property without value",
			cfg: `plugin {
						unknownTag
					}`,
			wantErr: true,
		},
		{
			name: "wrong field type",
			cfg: `plugin {
						int16num string
					}`,
			wantErr: true,
		},
		{
			name: "missing plugin body opener",
			cfg: `plugin
						tokenOnNextLine"`,
			wantErr: true,
		},
		{
			name:    "missing plugin body closure",
			cfg:     "plugin {",
			wantErr: true,
		},
		{
			name: "nonnull validation failed",
			cfg: `plugin {
							int16num 0
					}`,
			wantErr: true,
		},
		{
			name: "property is not a structure",
			cfg: `plugin {
						int16num {
							str
						}
					}`,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := caddy.NewTestController("dns", tt.cfg)
			ts := testStruct{}
			err := Parse(c, &ts)
			assert.Equalf(t, tt.wantErr, err != nil, "expected '%v' got '%v", tt.wantErr, err)
			if err == nil {
				assert.Equal(t, tt.want, ts)
			}
		})
	}
}

type person struct {
	Name    string        `cf:"name" default:"not-known"`
	Wife    *person       `cf:"wife"`
	Details personDetails `cf:"details"`
}

type personDetails struct {
	Age int `cf:"age" default:"17"`
}

func Test_ParseWithCaddy_NestedStructures(t *testing.T) {
	tests := []struct {
		name    string
		cfg     string
		want    person
		wantErr bool
	}{
		{
			name: "initialization of nested structure",
			cfg:  "plugin",
			want: person{Name: "not-known", Details: personDetails{Age: 17}},
		},
		{
			name: "initialization of dynamic nested structures if present",
			cfg: `plugin {
						wife {
						}
					}`,
			want: person{Name: "not-known", Wife: &person{Name: "not-known", Details: personDetails{Age: 17}}, Details: personDetails{Age: 17}},
		},
		{
			name: "specific value wins in nested structures over the defaults",
			cfg: `plugin {
						wife {
							name Maedl
						}
					}`,
			want: person{Name: "not-known", Wife: &person{Name: "Maedl", Details: personDetails{Age: 17}}, Details: personDetails{Age: 17}},
		},
		{
			name: "polyamory got into the tests ;)",
			cfg: `plugin {
						wife {
							name Maedl
							wife {
								name Birgit
								details {
									age 18
								}
							}
						}
					}`,
			want: person{Name: "not-known", Wife: &person{Name: "Maedl", Wife: &person{Name: "Birgit", Details: personDetails{Age: 18}}, Details: personDetails{Age: 17}}, Details: personDetails{Age: 17}},
		},
		{
			name: "error in nested structure is propagated",
			cfg: `plugin {
						nested {
							uknownProperty
						}
					}`,
			wantErr: true,
		},
		{
			name: "unknown structure field",
			cfg: `plugin {
						unknown {
							str string
						}
					}`,
			wantErr: true,
		},
		{
			name: "initialization of a nested structure needs body",
			cfg: `plugin {
						wife
					}`,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := caddy.NewTestController("dns", tt.cfg)
			ts := person{}
			err := Parse(c, &ts)
			assert.Equalf(t, tt.wantErr, err != nil, "expected '%v' got '%v", tt.wantErr, err)
			if err == nil {
				assert.Equal(t, tt.want, ts)
			}
		})
	}
}
