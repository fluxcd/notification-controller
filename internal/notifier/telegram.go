package notifier

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/containrrr/shoutrrr"
	eventv1 "github.com/fluxcd/pkg/apis/event/v1beta1"
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

func (t *Telegram) Post(ctx context.Context, event eventv1.Event) error {
	// Skip Git commit status update event.
	if event.HasMetadata(eventv1.MetaCommitStatusKey, eventv1.MetaCommitStatusUpdateValue) {
		return nil
	}

	emoji := "💫"
	if event.Severity == eventv1.EventSeverityError {
		emoji = "🚨"
	}

	heading := fmt.Sprintf("%s %s/%s/%s", emoji, strings.ToLower(event.InvolvedObject.Kind),
		event.InvolvedObject.Name, event.InvolvedObject.Namespace)
	var metadata string
	for k, v := range event.Metadata {
		metadata = metadata + fmt.Sprintf("\\- *%s*: %s\n", k, escapeString(v))
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
