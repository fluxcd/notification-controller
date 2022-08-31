package notifier

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/fluxcd/pkg/runtime/events"
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

func (l *Lark) Post(ctx context.Context, event events.Event) error {
	// Skip any update events
	if isCommitStatus(event.Metadata, "update") {
		return nil
	}

	emoji := "💫"
	color := "turquoise"
	if event.Severity == events.EventSeverityError {
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

	return postMessage(ctx, l.URL, "", nil, payload)
}
