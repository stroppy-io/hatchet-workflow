package library

import "time"

// parseDuration accepts Go durations ("2h30m") and bare hours ("2h").
func parseDuration(s string) (time.Duration, error) { return time.ParseDuration(s) }
