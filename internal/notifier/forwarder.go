package notifier

import (
	"fmt"
	"net/url"

	"github.com/fluxcd/pkg/recorder"
)

type Forwarder struct {
	URL      string
	ProxyURL string
}

func NewForwarder(hookURL string, proxyURL string) (*Forwarder, error) {
	if _, err := url.ParseRequestURI(hookURL); err != nil {
		return nil, fmt.Errorf("invalid Discord hook URL %s", hookURL)
	}

	return &Forwarder{
		URL:      hookURL,
		ProxyURL: proxyURL,
	}, nil
}

func (f *Forwarder) Post(event recorder.Event) error {
	err := postMessage(f.URL, f.ProxyURL, event)
	if err != nil {
		return fmt.Errorf("postMessage failed: %w", err)
	}
	return nil
}
