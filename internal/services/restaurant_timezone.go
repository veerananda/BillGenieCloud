package services

import (
	"errors"
	"os"
	"time"
)

const defaultRestaurantTimezone = "Asia/Kolkata"

// RestaurantLocation returns the timezone used for business-day boundaries (order history, counter tickets).
func RestaurantLocation() *time.Location {
	tz := os.Getenv("APP_TIMEZONE")
	if tz == "" {
		tz = defaultRestaurantTimezone
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return time.UTC
	}
	return loc
}

// StartOfRestaurantDay returns midnight at the start of the calendar day for t in the restaurant timezone.
func StartOfRestaurantDay(t time.Time) time.Time {
	loc := RestaurantLocation()
	inLoc := t.In(loc)
	y, m, d := inLoc.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, loc)
}

// ParseHistoryDateRange parses inclusive YYYY-MM-DD bounds into [from, toEnd) in the restaurant timezone.
func ParseHistoryDateRange(fromStr, toStr string) (from, toEnd time.Time, err error) {
	loc := RestaurantLocation()

	if fromStr == "" {
		now := time.Now().In(loc)
		from = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	} else {
		from, err = time.ParseInLocation("2006-01-02", fromStr, loc)
		if err != nil {
			return time.Time{}, time.Time{}, errors.New("invalid from date; use YYYY-MM-DD")
		}
	}

	if toStr == "" {
		toEnd = from.Add(24 * time.Hour)
	} else {
		var toDay time.Time
		toDay, err = time.ParseInLocation("2006-01-02", toStr, loc)
		if err != nil {
			return time.Time{}, time.Time{}, errors.New("invalid to date; use YYYY-MM-DD")
		}
		toEnd = toDay.Add(24 * time.Hour)
	}

	if !toEnd.After(from) {
		return time.Time{}, time.Time{}, errors.New("to must be on or after from")
	}

	return from, toEnd, nil
}

// historyActivityAtSQL is the timestamp used for order-history date filters (payment/completion, not kitchen bumps).
const historyActivityAtSQL = "COALESCE(completed_at, created_at)"
