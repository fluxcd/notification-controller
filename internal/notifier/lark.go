package notifier

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	eventv1 "github.com/fluxcd/pkg/apis/event/v1beta1"
)

type Lark struct {
	URL string
}

type LarkPayload struct {
	MsgType string   `json:"msg_type"`
	Card    LarkCard `json:"card"`
}

type LarkCard struct {
	Config LarkConfig `json:"config"`

	Header LarkHeader `json:"header"`

	Elements []LarkElement `json:"elements"`
}

type LarkConfig struct {
	WideScreenMode bool `json:"wide_screen_mode"`
}

type LarkHeader struct {
	Title    LarkTitle `json:"title"`
	Template string    `json:"template"`
}

type LarkTitle struct {
	Tag     string `json:"tag"`
	Content string `json:"content"`
}

type LarkElement struct {
	Tag  string   `json:"tag"`
	Text LarkText `json:"text"`
}

type LarkText struct {
	Tag     string `json:"tag"`
	Content string `json:"content"`
}

func NewLark(address string) (*Lark, error) {
	_, err := url.ParseRequestURI(address)
	if err != nil {
		return nil, fmt.Errorf("invalid Slack hook URL %s", address)
	}

	return &Lark{
		URL: address,
	}, nil
}

func (l *Lark) Post(ctx context.Context, event eventv1.Event) error {
	// Skip Git commit status update event.
	if event.HasMetadata(eventv1.MetaCommitStatusKey, eventv1.MetaCommitStatusUpdateValue) {
		return nil
	}

	emoji := "💫"
	color := "turquoise"
	if event.Severity == eventv1.EventSeverityError {
		emoji = "🚨"
		color = "red"
	}

	message := fmt.Sprintf("**%s**\n\n", event.Message)
	for k, v := range event.Metadata {
		message = message + fmt.Sprintf("%s: %s\n", k, v)
	}

	element := LarkElement{
		Tag: "div",
		Text: LarkText{
			Tag:     "lark_md",
			Content: message,
		},
	}

	card := LarkCard{
		Config: LarkConfig{
			WideScreenMode: true,
		},
		Header: LarkHeader{
			Title: LarkTitle{
				Tag: "plain_text",
				Content: fmt.Sprintf("%s %s/%s.%s", emoji, strings.ToLower(event.InvolvedObject.Kind),
					event.InvolvedObject.Name, event.InvolvedObject.Namespace),
			},
			Template: color,
		},
		Elements: []LarkElement{
			element,
		},
	}

	payload := LarkPayload{
		MsgType: "interactive",
		Card:    card,
	}

	return postMessage(ctx, l.URL, payload)
}
