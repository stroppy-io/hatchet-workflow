package api

import (
	"fmt"
	"time"

	"github.com/robfig/cron/v3"
)

// cronParser is the global cron expression parser used for suite
// scheduling. It accepts the standard 5-field POSIX syntax plus the
// "@hourly"/"@daily"/etc. descriptors. Seconds are deliberately not
// supported — suite firings are at minute granularity which is the
// natural fit for benchmark-style workloads.
var cronParser = cron.NewParser(
	cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor,
)

// nextCronFire parses expr and returns the next firing instant strictly
// after `from`. The caller is responsible for converting `from` into the
// suite's timezone before calling — the underlying cron schedule has no
// timezone of its own.
func nextCronFire(expr string, from time.Time) (time.Time, error) {
	sched, err := cronParser.Parse(expr)
	if err != nil {
		return time.Time{}, fmt.Errorf("cron: %w", err)
	}
	return sched.Next(from), nil
}
