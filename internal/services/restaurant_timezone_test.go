package services

import (
	"testing"
	"time"
)

func TestParseHistoryDateRangeYesterdayIST(t *testing.T) {
	t.Setenv("APP_TIMEZONE", "Asia/Kolkata")

	from, toEnd, err := ParseHistoryDateRange("2026-06-02", "2026-06-02")
	if err != nil {
		t.Fatalf("ParseHistoryDateRange: %v", err)
	}

	loc, _ := time.LoadLocation("Asia/Kolkata")
	wantFrom := time.Date(2026, 6, 2, 0, 0, 0, 0, loc)
	wantToEnd := time.Date(2026, 6, 3, 0, 0, 0, 0, loc)

	if !from.Equal(wantFrom) {
		t.Fatalf("from = %v, want %v", from, wantFrom)
	}
	if !toEnd.Equal(wantToEnd) {
		t.Fatalf("toEnd = %v, want %v", toEnd, wantToEnd)
	}

	paidAt := time.Date(2026, 6, 3, 14, 30, 0, 0, loc)
	if inHistoryRange(paidAt, from, toEnd) {
		t.Fatalf("midday June 3 order should not be in June 2 range")
	}
}

func TestParseHistoryDateRangeIncludesLateEveningIST(t *testing.T) {
	t.Setenv("APP_TIMEZONE", "Asia/Kolkata")

	from, toEnd, err := ParseHistoryDateRange("2026-06-02", "2026-06-02")
	if err != nil {
		t.Fatalf("ParseHistoryDateRange: %v", err)
	}

	loc, _ := time.LoadLocation("Asia/Kolkata")
	lateJune2 := time.Date(2026, 6, 2, 23, 30, 0, 0, loc)
	if !inHistoryRange(lateJune2, from, toEnd) {
		t.Fatalf("late June 2 order should be inside June 2 range")
	}
}

func inHistoryRange(at, from, toEnd time.Time) bool {
	return !at.Before(from) && at.Before(toEnd)
}
