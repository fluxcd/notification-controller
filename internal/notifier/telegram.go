package notifier

import (
	"errors"
	"fmt"
	"strings"

	"github.com/containrrr/shoutrrr"
	"github.com/fluxcd/pkg/runtime/events"
)

type Telegram struct {
	Channel string
	Token   string
}

func NewTelegram(channel, token string) (*Telegram, error) {
	if channel == "" {
		return nil, errors.New("empty Telegram channel")
	}

	return &Telegram{
		Channel: channel,
		Token:   token,
	}, nil
}

func (t *Telegram) Post(event events.Event) error {
	// Skip any update events
	if isCommitStatus(event.Metadata, "update") {
		return nil
	}

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
	url := fmt.Sprintf("telegram://%s@telegram?channels=%s&parseMode=markDownv2", t.Token, t.Channel)
	err := shoutrrr.Send(url, message)
	return err
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
