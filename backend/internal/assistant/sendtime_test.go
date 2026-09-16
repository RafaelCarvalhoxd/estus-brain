package assistant

import (
	"strings"
	"testing"
	"time"
)

func TestSendTimeLineDatesABacklockedMessageByWhenItWasSent(t *testing.T) {
	now := time.Date(2026, time.September, 15, 7, 0, 0, 0, time.UTC)
	got := sendTimeLine(now.Add(-3*time.Hour), now)
	want := "Esta mensagem ficou na fila e chegou atrasada: foi enviada em 2026-09-15 às 04:00. " +
		"Trate esse como o momento do pedido e passe a data 2026-09-15 nas ferramentas, em vez de deixar o padrão de hoje.\n"
	if got != want {
		t.Errorf("sendTimeLine() =\n%q\nwant\n%q", got, want)
	}
}

func TestSendTimeLineIsEmptyForAMessageThatJustArrived(t *testing.T) {
	now := time.Date(2026, time.September, 15, 7, 0, 0, 0, time.UTC)
	if got := sendTimeLine(now.Add(-30*time.Second), now); got != "" {
		t.Errorf("sendTimeLine(fresh) = %q, want empty", got)
	}
}

func TestSendTimeLineIsEmptyWhenTheSendTimeIsUnknown(t *testing.T) {
	now := time.Date(2026, time.September, 15, 7, 0, 0, 0, time.UTC)
	if got := sendTimeLine(time.Time{}, now); got != "" {
		t.Errorf("sendTimeLine(zero) = %q, want empty", got)
	}
}

// Telegram's clock running slightly ahead of ours must not read as a delay.
func TestSendTimeLineIsEmptyForASendTimeInTheFuture(t *testing.T) {
	now := time.Date(2026, time.September, 15, 7, 0, 0, 0, time.UTC)
	if got := sendTimeLine(now.Add(time.Minute), now); got != "" {
		t.Errorf("sendTimeLine(future) = %q, want empty", got)
	}
}

// The line is written in the owner's timezone, not the wire's UTC.
func TestSendTimeLineUsesTheClockTimezone(t *testing.T) {
	sp, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		t.Skip("no tzdata")
	}
	now := time.Date(2026, time.September, 15, 7, 0, 0, 0, sp)
	got := sendTimeLine(now.Add(-3*time.Hour).UTC(), now)
	if !strings.Contains(got, "às 04:00") {
		t.Errorf("sendTimeLine() = %q, want the São Paulo hour", got)
	}
}
