# corefile

## Description
Corefile package provides parsing of Corefile plugin configuration into a specified structure for custom CoreDNS plugins.
Parsing is driven by field tags in a structure provided to the parser as a target. That means that the only thing 
you have to do is to properly define a structure of your configuration, default values and validations. 

## Usage
The configuration might be parsed into a structure just by providing a specific structure instance as follows.
~~~
func setup(c *caddy.Controller) error {
	var cfg RedisConfig
	if err := corefile.Parse(c, &cfg); err != nil {
		return err
	}
	...
}
~~~

## Basic use case

The structure definition prescribes the structure of the plugin configuration. For each **exported** field in the structure a `cf` 
tag binds the structure field to a corresponding field in the configuration. The `cf` tag value is case-sensitive and 
represents the property name in the configuration.

You can provide default value for each field by using `default` tag. The value has to be compatible with the field type,
otherwise an error will be returned. The defaults are applied to structure fields when the structure is about to be 
parsed. They can be later overridden by parsed values. 

After the structure is parsed, validations are to be performed. For each field a several checks might be specified
by using `check` tag. The syntax and supported checks will described later.
~~~
type RedisConfig struct {
    Host string        `cf:"host" check:"nonempty"`
    Port string        `cf:"port" default:"1943" check:"nonempty,lt(5000)"`
    TTL  time.Duration `cf:"TTL" default:"30s"`
}
~~~

Here is how a configuration corresponding to the previously defined structure could look like.
~~~
redis {
    host redis.company.com
    port 1944
    TTL 10s
}
~~~

## Details 
### Supported field types
Currently, the following field types are supported:
* **string**
* **bool**
* numeric types
  * **int**, **int8**, **int16**, **int32**, **int64**
  * **float32**, **float64**
* slices
  * **[]string**
  * **[]int**
* **time.Duration**
* **net.IP**
* structs
* pointer to structs

### Plugin specific structure configuration

If a plugin configuration structure contains field name `Arguments` defined as `[]string`, it will be filled with 
plugin arguments provided in the configuration. If such field is not present in the plugin configuration structure, the
arguments will be ignored.

With the following configuration:
~~~
plugin arg1 arg2
~~~

the field Arguments will be filled by `["arg1", "arg2"]`:
~~~
type pluginCfg struct {
    Arguments []string
}
~~~

### Initialization

There are two possible ways to initialize fields in the structure before it is parsed: by default values, or by 
a structure initialization function.

#### Default values 

Specified by using `default` tag - the value has the same format as if the it would be provided in the configuration file

Example values:
~~~
    10s
    15.4
    "string value"
    true
    127.0.0.1
~~~

#### Specific structure initializer

If the structure implements `corefile.Initializer` interface, the method `Init() error` will be called immediately after 
the default values application.
    
This method of initialization is meant to be used for complex value initialization that is not possible via default 
values - like an initialization conditioned by some external factors.

In case of structure pointers, their initializer will be called when the parser enters the corresponding configuration 
block. If nested structure is not present in the config, the initialization of the corresponding nested structure won't be done.

Be aware, that the initialization function has to be implemented with pointer receiver. 
~~~
type pluginCfg struct {
    ...
}

func (v *pluginCfg) Init() error {
    ...
    return nil
}
~~~

### Validations

As same as the initialization, also validation can be done on a field level by providing a list a checkers, or on the structure level, by a structure specific checker function.

#### Field level checkers

There are several supported checkers, that can be applied on fields:

* **nonempty** - field must not have "default" value (for given golang type)
* **oneOf(arg1|...|argN)** - field must be one of specified values; it is applicable only on string fields
* **lt(arg)** - field value must be less than provided argument
* **lte(arg)** - field value must be less than or equal to provided argument
* **gt(arg)** - field value must be great than provided argument
* **gte(arg)** - field value must be great than or equal to provided argument

Checker's names are case-insensitive. You can specify several checkers in the `check` tag:
~~~
    age  `cf:"age" check:"gte(18),lt(100)"`
    city `cf:"city" check:"oneOf(Brno|Praha)"`
~~~

#### Custom structure validation

In case of complex validations the structure can implement `CustomChecker` interface. The `Check() error` function is called right after the field validations.

It can help in the situation where there are some relations between fields and simple field checkers are not sufficient.

~~~
type pluginCfg struct {
    ...
}

func (v *pluginCfg) Check() error {
    ...
    return nil
}
~~~

`Check()` function might be implemented on top of value as same as pointer receiver.

### Notes to structures

A configuration may refer another structures directly or by a pointer. 

When the configuration refers to another structure by pointer and such field is not initialized before the parsing, 
the memory for such field will be automatically allocated if the field is present in the configuration when the parser 
enters into the structure during the parsing.

Such field might be easily checked with `nonempty` checker to force its presence in the configuration, or left as is in 
case it is optional. In the following example, Regatta config is referred via pointer and its presence forced by usage 
of `nonempty` checker and the cache configuration directly via structure. The CacheCfg prescribes defaults where needed,
so the presence of the cache settings in the configuration is not necessary. 

~~~
type CacheCfg struct {
  Name            string        `cf:"name"`
  MaxSize         int           `cf:"maxSize" default:"1000"`
  TTL             time.Duration `cf:"ttl" default:"1m"`
  CleanupInterval time.Duration `cf:"cleanupInterval" default:"1m"`
  EvictType       string        `cf:"evictType" default:"arc" check:"oneOf(simple|lru|lfu|arc)"`
}

type Config struct {
  TLSCaCertFile  string       `cf:"tls-ca-cert-file" check:"nonempty"`
  Regatta        *grpc.Config `cf:"regatta" check:"nonempty"`
  Cache          CacheCfg     `cf:"customer-settings-cache"`
}
~~~
