package notifier

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	eventv1 "github.com/fluxcd/pkg/apis/event/v1beta1"
)

const (
	telegramBaseURL       = "https://api.telegram.org/bot%s"
	sendMessageMethodName = "sendMessage"
)

type Telegram struct {
	url      string
	ProxyURL string
	Channel  string
	Token    string
}

// TelegramPayload represents the payload sent to Telegram Bot API
// Reference: https://core.telegram.org/bots/api#sendmessage
type TelegramPayload struct {
	ChatID          string `json:"chat_id"`                     // Unique identifier for the target chat
	MessageThreadID string `json:"message_thread_id,omitempty"` // Unique identifier for the target message thread (topic) of the forum; for forum supergroups only
	Text            string `json:"text"`                        // Text of the message to be sent
	ParseMode       string `json:"parse_mode"`                  // Mode for parsing entities in the message text
}

func NewTelegram(proxyURL, channel, token string) (*Telegram, error) {
	if channel == "" {
		return nil, errors.New("empty Telegram channel")
	}

	if token == "" {
		return nil, errors.New("empty Telegram token")
	}

	return &Telegram{
		url:      fmt.Sprintf(telegramBaseURL, token),
		ProxyURL: proxyURL,
		Channel:  channel,
		Token:    token,
	}, nil
}

func (t *Telegram) Post(ctx context.Context, event eventv1.Event) error {
	emoji := "💫"
	if event.Severity == eventv1.EventSeverityError {
		emoji = "🚨"
	}

	heading := fmt.Sprintf("%s %s/%s/%s", emoji, strings.ToLower(event.InvolvedObject.Kind),
		event.InvolvedObject.Name, event.InvolvedObject.Namespace)
	var metadata string
	for k, v := range event.Metadata {
		metadata = metadata + fmt.Sprintf("\\- *%s*: %s\n", escapeString(k), escapeString(v))
	}
	message := fmt.Sprintf("*%s*\n%s\n%s", escapeString(heading), escapeString(event.Message), metadata)

	chatID, thread, channelHasThreadID := strings.Cut(t.Channel, ":")

	payload := TelegramPayload{
		ChatID:    chatID,
		Text:      message,
		ParseMode: "MarkdownV2", // https://core.telegram.org/bots/api#markdownv2-style
	}

	if channelHasThreadID {
		payload.MessageThreadID = thread
	}

	apiURL, err := url.JoinPath(t.url, sendMessageMethodName)
	if err != nil {
		return fmt.Errorf("failed to construct API URL: %w", err)
	}

	var opts []postOption
	if t.ProxyURL != "" {
		opts = append(opts, withProxy(t.ProxyURL))
	}

	return postMessage(ctx, apiURL, payload, opts...)
}

// The telegram API requires that some special characters are escaped
// in the message string. Docs: https://core.telegram.org/bots/api#formatting-options.
func escapeString(str string) string {
	chars := "\\.-_[]()~>`#+=|{}!*"
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
