package notifier

import (
	"fmt"
	"github.com/fluxcd/pkg/recorder"
	"net/url"
)

type Forwarder struct {
	URL string
}

func NewForwarder(hookURL string) (*Forwarder, error) {
	if _, err := url.ParseRequestURI(hookURL); err != nil {
		return nil, fmt.Errorf("invalid Discord hook URL %s", hookURL)
	}

	return &Forwarder{URL: hookURL}, nil
}

func (f *Forwarder) Post(event recorder.Event) error {
	err := postMessage(f.URL, event)
	if err != nil {
		return fmt.Errorf("postMessage failed: %w", err)
	}
	return nil
}
