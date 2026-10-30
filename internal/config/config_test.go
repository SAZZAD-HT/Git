package config

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"anvil/internal/errs"
)

func writeConfig(t *testing.T, c *Config) string {
	t.Helper()
	b, err := c.Render()
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(t.TempDir(), "anvil.toml")
	if err := os.WriteFile(p, b, 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestRenderRoundTrip(t *testing.T) {
	want := Default()
	want.Storage.DataDir = filepath.Join(t.TempDir(), `data dir`)
	want.Server.TrustedProxies = []string{"10.0.0.0/8"}
	got, err := Load(writeConfig(t, want))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("round trip mismatch\n got %+v\nwant %+v", got, want)
	}
}

func TestRelativeDataDirResolvesAgainstConfigFile(t *testing.T) {
	p := writeConfig(t, Default())
	got, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(filepath.Dir(p), "data"); got.Storage.DataDir != want {
		t.Fatalf("data_dir = %q, want %q", got.Storage.DataDir, want)
	}
}

func TestLoadReportsEveryProblem(t *testing.T) {
	p := filepath.Join(t.TempDir(), "anvil.toml")
	body := "[server]\ninternal_addr = \"0.0.0.0:3001\"\nbogus = 1\n[log]\nlevel = \"loud\"\n"
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := Load(p)
	if !errors.Is(err, errs.Invalid) {
		t.Fatalf("want errs.Invalid, got %v", err)
	}
	for _, k := range []string{"server.internal_addr", "server.bogus", "log.level"} {
		if !strings.Contains(err.Error(), k) {
			t.Errorf("error does not mention %s: %v", k, err)
		}
	}
}

func TestMissingFile(t *testing.T) {
	_, err := Load(filepath.Join(t.TempDir(), "nope.toml"))
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("want os.ErrNotExist, got %v", err)
	}
}

// TC-NFR-12: every key can be overridden by an environment variable.
func TestEveryKeyHasEnvOverride(t *testing.T) {
	values := map[reflect.Kind]string{reflect.String: "x", reflect.Bool: "true", reflect.Int: "7", reflect.Slice: "a, b"}
	env := []string{}
	cfg := Default()
	forEachKey(cfg, func(section, key string, f reflect.Value) {
		v := values[f.Kind()]
		switch f.Addr().Interface().(type) {
		case *Duration:
			v = "42s"
		case *ByteSize:
			v = "3MiB"
		}
		env = append(env, EnvName(section, key)+"="+v)
	})
	if problems := applyEnv(cfg, env); len(problems) > 0 {
		t.Fatal(problems)
	}
	forEachKey(cfg, func(section, key string, f reflect.Value) {
		var ok bool
		switch v := f.Addr().Interface().(type) {
		case *Duration:
			ok = v.Duration == 42*time.Second
		case *ByteSize:
			ok = *v == 3<<20
		case *string:
			ok = *v == "x"
		case *bool:
			ok = *v
		case *int:
			ok = *v == 7
		case *[]string:
			ok = reflect.DeepEqual(*v, []string{"a", "b"})
		}
		if !ok {
			t.Errorf("%s was not overridden (value %v)", EnvName(section, key), f.Interface())
		}
	})
}

func TestEnvNamesAreCaseInsensitive(t *testing.T) {
	cfg := Default()
	applyEnv(cfg, []string{"anvil_log_level=debug"})
	if cfg.Log.Level != "debug" {
		t.Fatalf("level = %q", cfg.Log.Level)
	}
}

// docs/reference/15 §1: every Config key appears in the configuration reference.
func TestEveryKeyIsDocumented(t *testing.T) {
	doc, err := os.ReadFile(filepath.Join("..", "..", "docs", "reference", "11-configuration-and-operations.md"))
	if err != nil {
		t.Fatal(err)
	}
	section := ""
	documented := map[string]bool{}
	for _, line := range strings.Split(string(doc), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.Trim(line, "[]")
		} else if k, _, ok := strings.Cut(line, "="); ok && section != "" {
			documented[section+"."+strings.TrimSpace(k)] = true
		}
	}
	forEachKey(Default(), func(section, key string, _ reflect.Value) {
		if !documented[section+"."+key] {
			t.Errorf("%s.%s is not documented in docs/reference/11 §3", section, key)
		}
	})
}

func TestByteSize(t *testing.T) {
	for in, want := range map[string]ByteSize{"512": 512, "64KiB": 64 << 10, "2GiB": 2 << 30, "1 MiB": 1 << 20} {
		var s ByteSize
		if err := s.UnmarshalText([]byte(in)); err != nil || s != want {
			t.Errorf("%q -> %d, %v; want %d", in, s, err, want)
		}
	}
	var s ByteSize
	if err := s.UnmarshalText([]byte("2GB")); err == nil {
		t.Error("decimal unit accepted")
	}
}
