package config

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Duration is a time.Duration written in config files as a Go duration string ("30s", "336h").
type Duration struct{ time.Duration }

// UnmarshalText parses a Go duration string such as "1h30m".
func (d *Duration) UnmarshalText(b []byte) error {
	v, err := time.ParseDuration(strings.TrimSpace(string(b)))
	if err != nil {
		return fmt.Errorf("invalid duration %q (examples: 30s, 5m, 1h)", string(b))
	}
	d.Duration = v
	return nil
}

// MarshalText formats the duration the way UnmarshalText reads it.
func (d Duration) MarshalText() ([]byte, error) { return []byte(d.Duration.String()), nil }

// ByteSize is a size in bytes written with a binary unit suffix ("50MiB", "2GiB").
// A bare number is bytes.
type ByteSize int64

var byteUnits = []struct {
	suffix string
	mult   int64
}{{"KiB", 1 << 10}, {"MiB", 1 << 20}, {"GiB", 1 << 30}, {"TiB", 1 << 40}, {"B", 1}}

// UnmarshalText parses sizes such as "512", "64KiB", "50MiB" or "2GiB".
// Decimal units (MB, GB) are rejected on purpose: they are ambiguous.
func (s *ByteSize) UnmarshalText(b []byte) error {
	text := strings.TrimSpace(string(b))
	mult := int64(1)
	num := text
	for _, u := range byteUnits {
		if strings.HasSuffix(text, u.suffix) {
			mult, num = u.mult, strings.TrimSpace(strings.TrimSuffix(text, u.suffix))
			break
		}
	}
	n, err := strconv.ParseInt(num, 10, 64)
	if err != nil || n < 0 {
		return fmt.Errorf("invalid size %q (examples: 512KiB, 50MiB, 2GiB)", text)
	}
	*s = ByteSize(n * mult)
	return nil
}

// MarshalText writes the size with the largest unit that divides it exactly.
func (s ByteSize) MarshalText() ([]byte, error) {
	n := int64(s)
	for _, u := range byteUnits[:4] {
		if n != 0 && n%u.mult == 0 && n/u.mult < 1<<10 {
			return []byte(strconv.FormatInt(n/u.mult, 10) + u.suffix), nil
		}
	}
	return []byte(strconv.FormatInt(n, 10)), nil
}
