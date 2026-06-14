package interp

import (
	"context"
	"fmt"
	"net/mail"
	"strconv"
	"strings"
	"time"
)

// DatePart identifies which component of a date-time value to extract.
type DatePart string

const (
	DatePartYear    DatePart = "year"
	DatePartMonth   DatePart = "month"
	DatePartDay     DatePart = "day"
	DatePartDate    DatePart = "date"
	DatePartJulian  DatePart = "julian"
	DatePartHour    DatePart = "hour"
	DatePartMinute  DatePart = "minute"
	DatePartSecond  DatePart = "second"
	DatePartTime    DatePart = "time"
	DatePartISO8601 DatePart = "iso8601"
	DatePartStd11   DatePart = "std11"
	DatePartZone    DatePart = "zone"
	DatePartWeekday DatePart = "weekday"
)

var validDateParts = map[DatePart]struct{}{
	DatePartYear:    {},
	DatePartMonth:   {},
	DatePartDay:     {},
	DatePartDate:    {},
	DatePartJulian:  {},
	DatePartHour:    {},
	DatePartMinute:  {},
	DatePartSecond:  {},
	DatePartTime:    {},
	DatePartISO8601: {},
	DatePartStd11:   {},
	DatePartZone:    {},
	DatePartWeekday: {},
}

func extractDatePart(t time.Time, part DatePart) (string, error) {
	switch part {
	case DatePartYear:
		return strconv.Itoa(t.Year()), nil
	case DatePartMonth:
		return fmt.Sprintf("%02d", int(t.Month())), nil
	case DatePartDay:
		return fmt.Sprintf("%02d", t.Day()), nil
	case DatePartDate:
		return t.Format("2006-01-02"), nil
	case DatePartJulian:
		return strconv.Itoa(modifiedJulianDay(t)), nil
	case DatePartHour:
		return fmt.Sprintf("%02d", t.Hour()), nil
	case DatePartMinute:
		return fmt.Sprintf("%02d", t.Minute()), nil
	case DatePartSecond:
		return fmt.Sprintf("%02d", t.Second()), nil
	case DatePartTime:
		return t.Format("15:04:05"), nil
	case DatePartISO8601:
		return t.Format("2006-01-02T15:04:05-07:00"), nil
	case DatePartStd11:
		return t.Format(time.RFC1123Z), nil
	case DatePartZone:
		return t.Format("-0700"), nil
	case DatePartWeekday:
		return strconv.Itoa(int(t.Weekday())), nil
	default:
		return "", fmt.Errorf("unknown date-part: %s", part)
	}
}

// modifiedJulianDay calculates days since 1858-11-17 00:00 UTC.
func modifiedJulianDay(t time.Time) int {
	year, month, day := t.Date()
	m := int(month)
	a := (14 - m) / 12
	y := year + 4800 - a
	m = m + 12*a - 3
	jdn := day + (153*m+2)/5 + 365*y + y/4 - y/100 + y/400 - 32045
	return jdn - 2400001
}

func parseZoneOffset(zone string) (int, error) {
	if len(zone) != 5 {
		return 0, fmt.Errorf("invalid zone format: %s", zone)
	}
	sign := 1
	switch zone[0] {
	case '-':
		sign = -1
	case '+':
	default:
		return 0, fmt.Errorf("invalid zone format: %s", zone)
	}
	hours, err := strconv.Atoi(zone[1:3])
	if err != nil {
		return 0, fmt.Errorf("invalid zone hours: %s", zone)
	}
	minutes, err := strconv.Atoi(zone[3:5])
	if err != nil {
		return 0, fmt.Errorf("invalid zone minutes: %s", zone)
	}
	return sign * (hours*3600 + minutes*60), nil
}

var dateFormats = []string{
	time.RFC1123Z,
	time.RFC1123,
	time.RFC822Z,
	time.RFC822,
	time.RFC3339,
	time.RFC3339Nano,
	"Mon, 2 Jan 2006 15:04:05 -0700",
	"Mon, 2 Jan 2006 15:04:05 MST",
	"2 Jan 2006 15:04:05 -0700",
	"2 Jan 2006 15:04:05 MST",
}

func parseDateHeader(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, fmt.Errorf("empty date value")
	}
	if t, err := mail.ParseDate(value); err == nil {
		return t, nil
	}
	for _, format := range dateFormats {
		if t, err := time.Parse(format, value); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unable to parse date: %s", value)
}

// DateTest implements the RFC 5260 "date" test.
type DateTest struct {
	matcherTest

	Header       string
	DatePart     DatePart
	Zone         string
	OriginalZone bool
	Index        int
	Last         bool
}

func (d DateTest) Check(_ context.Context, rd *RuntimeData) (bool, error) {
	header := expandVars(rd, d.Header)
	values, err := rd.Msg.HeaderGet(header)
	if err != nil {
		return false, err
	}

	if d.isCount() {
		var cnt uint64
		for _, v := range values {
			if _, e := parseDateHeader(v); e == nil {
				cnt++
			}
		}
		return d.countMatches(rd, cnt), nil
	}

	if len(values) == 0 {
		return false, nil
	}

	var value string
	if d.Index > 0 {
		idx := d.Index - 1
		if d.Last {
			idx = len(values) - d.Index
		}
		if idx < 0 || idx >= len(values) {
			return false, nil
		}
		value = values[idx]
	} else {
		value = values[0]
	}

	t, err := parseDateHeader(value)
	if err != nil {
		return false, nil
	}

	t = applyZone(t, d.Zone, d.OriginalZone)

	datePart := DatePart(strings.ToLower(expandVars(rd, string(d.DatePart))))
	partValue, err := extractDatePart(t, datePart)
	if err != nil {
		return false, err
	}

	return d.matcherTest.tryMatch(rd, partValue)
}

// CurrentDateTest implements the RFC 5260 "currentdate" test.
type CurrentDateTest struct {
	matcherTest

	DatePart DatePart
	Zone     string
}

func (c CurrentDateTest) Check(_ context.Context, rd *RuntimeData) (bool, error) {
	t := time.Now()
	if c.Zone != "" {
		if offset, err := parseZoneOffset(c.Zone); err == nil {
			t = t.In(time.FixedZone("", offset))
		}
	}

	datePart := DatePart(strings.ToLower(expandVars(rd, string(c.DatePart))))
	partValue, err := extractDatePart(t, datePart)
	if err != nil {
		return false, err
	}

	return c.matcherTest.tryMatch(rd, partValue)
}

func applyZone(t time.Time, zone string, original bool) time.Time {
	if original {
		return t
	}
	if zone != "" {
		if offset, err := parseZoneOffset(zone); err == nil {
			return t.In(time.FixedZone("", offset))
		}
	}
	return t.Local()
}
