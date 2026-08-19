package config

import "time"

// duration wraps time.Duration and implements encoding.TextUnmarshaler, the interface
// gonfig uses to parse struct fields beyond the native Go types. Without this, a plain
// time.Duration field would be parsed as a raw int64 (nanoseconds), forcing config
// values like NODE_WATCHDOG_TIMEOUT_DURATION to be specified in nanoseconds instead of
// as "5m" or "30s". It exists purely to feed the raw flags struct in config.go; Load
// converts it to a plain time.Duration in the Configuration it returns, so nothing
// outside this package ever sees this type.
type duration time.Duration

func (d *duration) UnmarshalText(text []byte) error {
	parsed, err := time.ParseDuration(string(text))
	if err != nil {
		return err
	}

	*d = duration(parsed)
	return nil
}

func (d duration) String() string {
	return time.Duration(d).String()
}
