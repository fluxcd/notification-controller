package notifier

import (
	"fmt"

	"github.com/containrrr/shoutrrr"
	"github.com/fluxcd/pkg/runtime/events"

	"github.com/fluxcd/notification-controller/api/v1beta1"
)

type Shoutrrr struct {
	URL  string
	Type string
}

func (s *Shoutrrr) getMessage(event events.Event) (string, error) {
	var message string
	var err error
	switch s.Type {
	case v1beta1.TelegramProvider:
		message = TelegramMessage(event)
	default:
		err = fmt.Errorf("provider currently not supported by shoutrrr")
	}

	return message, err
}

func (s *Shoutrrr) Post(event events.Event) error {
	// Skip any update events
	if isCommitStatus(event.Metadata, "update") {
		return nil
	}

	msg, err := s.getMessage(event)
	if err != nil {
		return err
	}

	return shoutrrr.Send(s.URL, msg)
}
