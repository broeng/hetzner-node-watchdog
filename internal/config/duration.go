package config

import "time"

// Duration wraps time.Duration and implements encoding.TextUnmarshaler, the interface
// gonfig uses to parse struct fields beyond the native Go types. Without this, a plain
// time.Duration field would be parsed as a raw int64 (nanoseconds), forcing config
// values like NODE_WATCHDOG_TIMEOUT_DURATION to be specified in nanoseconds instead of
// as "5m" or "30s".
type Duration time.Duration

func (d *Duration) UnmarshalText(text []byte) error {
	parsed, err := time.ParseDuration(string(text))
	if err != nil {
		return err
	}

	*d = Duration(parsed)
	return nil
}

func (d Duration) String() string {
	return time.Duration(d).String()
}
