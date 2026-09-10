package schedule

import (
	"time"

	"github.com/robfig/cron/v3"
)

// cronParser accepts the standard five fields (minute hour dom month dow)
// and the @hourly / @daily / @weekly / @monthly descriptors; the
// timezone comes from the schedule, not from the expression.
//
// doc: pkg.go.dev/github.com/robfig/cron/v3 — Parser, Schedule.Next.
var cronParser = cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)

// Cron is a parsed expression.
type Cron struct {
	schedule cron.Schedule
	expr     string
}

// String returns the source expression.
func (c Cron) String() string { return c.expr }

// ParseCron parses an expression.
func ParseCron(expr string) (Cron, error) {
	s, err := cronParser.Parse(expr)
	if err != nil {
		return Cron{}, err
	}
	return Cron{schedule: s, expr: expr}, nil
}

// Next is the first firing strictly after t, in t's location.
func (c Cron) Next(t time.Time) time.Time { return c.schedule.Next(t) }
