package notifier

import (
	"errors"
	"fmt"
	"strings"

	"github.com/fluxcd/pkg/runtime/events"

	"github.com/fluxcd/notification-controller/api/v1beta1"
)

type Telegram struct {
	Channel string
	Token   string
}

func NewTelegram(channel, token string) (*Shoutrrr, error) {
	if channel == "" {
		return nil, errors.New("empty Telegram channel")
	}

	url := fmt.Sprintf("telegram://%s@telegram?channels=%s&parseMode=markDownv2",
		token, channel)
	return &Shoutrrr{
		URL:  url,
		Type: v1beta1.TelegramProvider,
	}, nil
}

func TelegramMessage(event events.Event) string {
	emoji := "💫"
	if event.Severity == events.EventSeverityError {
		emoji = "🚨"
	}

	heading := fmt.Sprintf("%s %s/%s/%s", emoji, strings.ToLower(event.InvolvedObject.Kind),
		event.InvolvedObject.Name, event.InvolvedObject.Namespace)
	var metadata string
	for k, v := range event.Metadata {
		metadata = metadata + fmt.Sprintf("\\- *%s*: %s\n", k, v)
	}
	message := fmt.Sprintf("*%s*\n%s\n%s", escapeString(heading), escapeString(event.Message), metadata)

	return message
}

// The telegram API requires that some special characters are escaped
// in the message string. Docs: https://core.telegram.org/bots/api#formatting-options.
func escapeString(str string) string {
	chars := "\\.-_[]()~>`#+=|{}!"
	for _, char := range chars {
		start := 0
		idx := 0
		for start < len(str) && idx != -1 {
			idx = strings.IndexRune(str[start:], char)
			if idx != -1 {
				newIdx := start + idx
				str = str[:newIdx] + `\` + str[newIdx:]
				start = newIdx + 2
			}
		}
	}

	return str
}
