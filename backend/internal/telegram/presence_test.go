package telegram

import (
	"testing"
	"time"
)

func TestConnectedNoticeSaysHowLongTheBotWasAway(t *testing.T) {
	now := time.Date(2026, time.September, 15, 7, 0, 0, 0, saoPaulo(t))
	since := now.Add(-2*time.Hour - 10*time.Minute)
	got := connectedNotice(&since, now)
	want := "🟢 <b>Estus conectado</b>\nEstive fora desde 15/09 04:50 (2h10)."
	if got != want {
		t.Errorf("connectedNotice() =\n%q\nwant\n%q", got, want)
	}
}

func TestConnectedNoticeWithoutAKnownAbsence(t *testing.T) {
	now := time.Date(2026, time.September, 15, 7, 0, 0, 0, saoPaulo(t))
	got := connectedNotice(nil, now)
	want := "🟢 <b>Estus conectado</b>"
	if got != want {
		t.Errorf("connectedNotice(nil) = %q, want %q", got, want)
	}
}

// A clock that ran backwards, or a last_online_at written a moment ago by a
// process that is still shutting down, must not produce a negative gap.
func TestConnectedNoticeIgnoresAnAbsenceInTheFuture(t *testing.T) {
	now := time.Date(2026, time.September, 15, 7, 0, 0, 0, saoPaulo(t))
	since := now.Add(time.Minute)
	got := connectedNotice(&since, now)
	want := "🟢 <b>Estus conectado</b>"
	if got != want {
		t.Errorf("connectedNotice(future) = %q, want %q", got, want)
	}
}

func TestAwayFor(t *testing.T) {
	cases := []struct {
		d    time.Duration
		want string
	}{
		{40 * time.Second, "menos de 1 min"},
		{time.Minute, "1 min"},
		{45 * time.Minute, "45 min"},
		{time.Hour, "1h00"},
		{2*time.Hour + 10*time.Minute, "2h10"},
		{26 * time.Hour, "1d 2h"},
		{50 * time.Hour, "2d 2h"},
	}
	for _, tc := range cases {
		if got := awayFor(tc.d); got != tc.want {
			t.Errorf("awayFor(%v) = %q, want %q", tc.d, got, tc.want)
		}
	}
}

func TestDisconnectedNoticeCarriesTheReason(t *testing.T) {
	got := disconnectedNotice("Desligando.")
	want := "🔴 <b>Estus desconectado</b>\nDesligando."
	if got != want {
		t.Errorf("disconnectedNotice() = %q, want %q", got, want)
	}
}

func TestDisconnectedNoticeEscapesTheReason(t *testing.T) {
	got := disconnectedNotice("Outro <Estus> está usando este bot")
	want := "🔴 <b>Estus desconectado</b>\nOutro &lt;Estus&gt; está usando este bot"
	if got != want {
		t.Errorf("disconnectedNotice() = %q, want %q", got, want)
	}
}
