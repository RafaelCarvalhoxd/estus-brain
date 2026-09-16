package assistant

import (
	"fmt"
	"time"
)

// lateAfter is how far behind a message has to be before the assistant is
// told about the delay. Below it the gap is just the time an answer takes,
// and saying so would only add noise to every prompt.
const lateAfter = 2 * time.Minute

// sendTimeLine tells the assistant when a message was actually sent, for the
// messages that waited. Telegram holds what the owner writes for up to 24h
// while the bot is off and hands the backlog over at once when it returns:
// without this, "gastei 50 no mercado" said on Tuesday is booked on the day
// the laptop came back. An unknown or future send time gives no line at all.
func sendTimeLine(sentAt, now time.Time) string {
	if sentAt.IsZero() || now.Sub(sentAt) < lateAfter {
		return ""
	}
	at := sentAt.In(now.Location())
	return fmt.Sprintf("Esta mensagem ficou na fila e chegou atrasada: foi enviada em %s às %s. "+
		"Trate esse como o momento do pedido e passe a data %s nas ferramentas, em vez de deixar o padrão de hoje.\n",
		at.Format(dayLayout), at.Format("15:04"), at.Format(dayLayout))
}
