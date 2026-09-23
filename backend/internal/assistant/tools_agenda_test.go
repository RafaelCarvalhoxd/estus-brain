package assistant

import (
	"testing"
	"time"

	"github.com/rafael/estus-vault/backend/internal/domain"
)

func TestEventRangeOptionalTimes(t *testing.T) {
	loc, _ := time.LoadLocation("America/Sao_Paulo")
	day := time.Date(2026, 9, 22, 0, 0, 0, 0, time.UTC)
	cases := []struct {
		start, end string
		minutes    int
		want       [2]string
	}{
		{"", "", 0, [2]string{"00:00", "23:59"}},
		{"14:30", "", 0, [2]string{"14:30", "15:30"}},
		{"23:30", "", 0, [2]string{"23:30", "23:59"}},
		{"", "10:00", 0, [2]string{"09:00", "10:00"}},
		{"", "00:30", 0, [2]string{"00:00", "00:30"}},
		{"9h", "", 30, [2]string{"09:00", "09:30"}},
		{"9h30", "11:00", 0, [2]string{"09:30", "11:00"}},
	}
	for _, c := range cases {
		s, e, err := eventRange(day, c.start, c.end, c.minutes, loc)
		if err != nil {
			t.Errorf("eventRange(%q, %q): %v", c.start, c.end, err)
			continue
		}
		if s.Location() != loc || s.Format("2006-01-02") != "2026-09-22" {
			t.Errorf("eventRange(%q, %q) start = %v, want 2026-09-22 in owner tz", c.start, c.end, s)
		}
		if got := [2]string{s.Format("15:04"), e.Format("15:04")}; got != c.want {
			t.Errorf("eventRange(%q, %q) = %v, want %v", c.start, c.end, got, c.want)
		}
	}
	for _, bad := range [][2]string{{"15:00", "14:00"}, {"25:00", ""}, {"manhã", ""}} {
		if _, _, err := eventRange(day, bad[0], bad[1], 0, loc); err == nil {
			t.Errorf("eventRange(%q, %q) accepted", bad[0], bad[1])
		}
	}
}

func TestSplitEventDate(t *testing.T) {
	cases := []struct{ date, start, wantDate, wantStart string }{
		{"2026-09-22", "10:00", "2026-09-22", "10:00"},
		{"", "2026-09-22T10:00", "2026-09-22", "10:00"},
		{"2026-09-22T10:00", "", "2026-09-22", "10:00"},
		{"2026-09-22 10:00", "11:00", "2026-09-22", "11:00"},
		{"amanhã", "", "amanhã", ""},
	}
	for _, c := range cases {
		d, s := splitEventDate(c.date, c.start)
		if d != c.wantDate || s != c.wantStart {
			t.Errorf("splitEventDate(%q, %q) = %q, %q", c.date, c.start, d, s)
		}
	}
}

func TestMoveEvent(t *testing.T) {
	loc, _ := time.LoadLocation("America/Sao_Paulo")
	at := func(day, clock string) time.Time {
		v, err := time.ParseInLocation("2006-01-02 15:04", day+" "+clock, loc)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	e := domain.Event{StartsAt: at("2026-09-22", "10:00"), EndsAt: at("2026-09-22", "11:30")}
	newDay := time.Date(2026, 9, 25, 0, 0, 0, 0, time.UTC)
	cases := []struct {
		name       string
		day        *time.Time
		start, end string
		allDay     bool
		want       [2]string
	}{
		{"nothing keeps times", nil, "", "", false, [2]string{"2026-09-22 10:00", "2026-09-22 11:30"}},
		{"new day keeps times", &newDay, "", "", false, [2]string{"2026-09-25 10:00", "2026-09-25 11:30"}},
		{"new start keeps duration", nil, "15:00", "", false, [2]string{"2026-09-22 15:00", "2026-09-22 16:30"}},
		{"late start stops at day end", nil, "23:00", "", false, [2]string{"2026-09-22 23:00", "2026-09-22 23:59"}},
		{"new end keeps start", nil, "", "12:00", false, [2]string{"2026-09-22 10:00", "2026-09-22 12:00"}},
		{"both", &newDay, "08:00", "09:00", false, [2]string{"2026-09-25 08:00", "2026-09-25 09:00"}},
		{"all day", &newDay, "", "", true, [2]string{"2026-09-25 00:00", "2026-09-25 23:59"}},
	}
	for _, c := range cases {
		s, en, err := moveEvent(e, c.day, c.start, c.end, c.allDay, loc)
		if err != nil {
			t.Errorf("%s: %v", c.name, err)
			continue
		}
		if got := [2]string{s.Format("2006-01-02 15:04"), en.Format("2006-01-02 15:04")}; got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
	if _, _, err := moveEvent(e, nil, "", "09:00", false, loc); err == nil {
		t.Error("end before the kept start was accepted")
	}
	if !isAllDay(domain.Event{StartsAt: at("2026-09-22", "00:00"), EndsAt: at("2026-09-22", "23:59")}, loc) || isAllDay(e, loc) {
		t.Error("isAllDay misclassified")
	}
}
