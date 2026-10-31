package config

import (
	"encoding"
	"reflect"
	"strconv"
	"strings"
)

// EnvPrefix starts every environment override: ANVIL_<SECTION>_<KEY>.
const EnvPrefix = "ANVIL_"

// EnvName returns the environment variable that overrides section.key,
// e.g. EnvName("server", "http_addr") == "ANVIL_SERVER_HTTP_ADDR".
func EnvName(section, key string) string {
	return EnvPrefix + strings.ToUpper(section) + "_" + strings.ToUpper(key)
}

// applyEnv overwrites fields of cfg from env (a list of KEY=VALUE, as os.Environ returns).
// Lists are comma-separated; an empty value gives an empty list. It returns a problem
// message per "section.key" that could not be parsed.
//
// Variables that start with ANVIL_ but match no key are ignored rather than rejected,
// because the same prefix is used for hook and runner variables (ANVIL_TRACE_ID, …).
func applyEnv(cfg *Config, env []string) map[string]string {
	vars := make(map[string]string, len(env))
	for _, kv := range env {
		if k, v, ok := strings.Cut(kv, "="); ok && strings.HasPrefix(strings.ToUpper(k), EnvPrefix) {
			vars[strings.ToUpper(k)] = v // Windows env names are case-insensitive
		}
	}
	problems := map[string]string{}
	forEachKey(cfg, func(section, key string, field reflect.Value) {
		raw, ok := vars[EnvName(section, key)]
		if !ok {
			return
		}
		if err := setFromString(field, raw); err != nil {
			problems[section+"."+key] = "from " + EnvName(section, key) + ": " + err.Error()
		}
	})
	return problems
}

// forEachKey calls fn for every leaf key of cfg with its TOML section and key names.
func forEachKey(cfg *Config, fn func(section, key string, field reflect.Value)) {
	root := reflect.ValueOf(cfg).Elem()
	for i := 0; i < root.NumField(); i++ {
		section := root.Type().Field(i).Tag.Get("toml")
		sv := root.Field(i)
		for j := 0; j < sv.NumField(); j++ {
			fn(section, sv.Type().Field(j).Tag.Get("toml"), sv.Field(j))
		}
	}
}

var textUnmarshaler = reflect.TypeFor[encoding.TextUnmarshaler]()

// setFromString parses raw into field according to the field's type.
func setFromString(field reflect.Value, raw string) error {
	if field.Addr().Type().Implements(textUnmarshaler) {
		return field.Addr().Interface().(encoding.TextUnmarshaler).UnmarshalText([]byte(raw))
	}
	switch field.Kind() {
	case reflect.String:
		field.SetString(raw)
	case reflect.Bool:
		b, err := strconv.ParseBool(strings.TrimSpace(raw))
		if err != nil {
			return err
		}
		field.SetBool(b)
	case reflect.Int, reflect.Int64:
		n, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
		if err != nil {
			return err
		}
		field.SetInt(n)
	case reflect.Slice:
		items := []string{}
		for _, s := range strings.Split(raw, ",") {
			if s = strings.TrimSpace(s); s != "" {
				items = append(items, s)
			}
		}
		field.Set(reflect.ValueOf(items))
	default:
		panic("config: unsupported field kind " + field.Kind().String()) // programming error, caught by tests
	}
	return nil
}
