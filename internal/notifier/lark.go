package notifier

import "github.com/fluxcd/pkg/runtime/events"

type Lark struct {
	URL string
}

type LarkPayload struct {
	MsgType string      `json:"msg_type"`
	Content LarkContent `json:"content"`
}

type LarkContent struct {
	Text string `json:"text"`
}

func NewLark(address string) *Lark {
	return &Lark{
		URL: address,
	}
}

func (l *Lark) Post(event events.Event) error {
	// Skip any update events
	if isCommitStatus(event.Metadata, "update") {
		return nil
	}

	payload := LarkPayload{
		MsgType: "text",
		Content: LarkContent{
			Text: event.Message,
		},
	}

	return postMessage(l.URL, "", nil, payload)
}
