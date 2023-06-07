/*
Copyright 2020 The Flux authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package notifier

import (
	"context"
	"fmt"
	"strings"

	eventv1 "github.com/fluxcd/pkg/apis/event/v1beta1"
)

// Slack holds the hook URL
type GoogleChat struct {
	URL      string
	ProxyURL string
	Username string
	Channel  string
}

// GoogleChatPayload holds the channel and attachments
type GoogleChatPayload struct {
	Cards []GoogleChatCard `json:"cards"`
}

type GoogleChatCard struct {
	Header   GoogleChatCardHeader    `json:"header"`
	Sections []GoogleChatCardSection `json:"sections"`
}

type GoogleChatCardHeader struct {
	Title      string  `json:"title"`
	SubTitle   string  `json:"subtitle"`
	ImageUrl   *string `json:"imageUrl"`
	ImageStyle *string `json:"imageStyle"`
}

type GoogleChatCardSection struct {
	Header  string                 `json:"header"`
	Widgets []GoogleChatCardWidget `json:"widgets"`
}

type GoogleChatCardWidget struct {
	TextParagraph *GoogleChatCardWidgetTextParagraph `json:"textParagraph"`
	KeyValue      *GoogleChatCardWidgetKeyValue      `json:"keyValue"`
}

type GoogleChatCardWidgetTextParagraph struct {
	Text string `json:"text"`
}

type GoogleChatCardWidgetKeyValue struct {
	TopLabel         string  `json:"topLabel"`
	Content          string  `json:"content"`
	ContentMultiLine bool    `json:"contentMultiline"`
	BottomLabel      *string `json:"bottomLabel"`
	Icon             *string `json:"icon"`
}

// NewGoogleChat validates the Google Chat URL and returns a GoogleChat object
func NewGoogleChat(hookURL string, proxyURL string) (*GoogleChat, error) {
	return &GoogleChat{
		URL:      hookURL,
		ProxyURL: proxyURL,
	}, nil
}

// Post Google Chat message
func (s *GoogleChat) Post(ctx context.Context, event eventv1.Event) error {
	// Skip Git commit status update event.
	if event.HasMetadata(eventv1.MetaCommitStatusKey, eventv1.MetaCommitStatusUpdateValue) {
		return nil
	}

	// Header
	objName := fmt.Sprintf("%s/%s.%s", strings.ToLower(event.InvolvedObject.Kind), event.InvolvedObject.Name, event.InvolvedObject.Namespace)
	header := GoogleChatCardHeader{
		Title:    objName,
		SubTitle: event.ReportingController,
	}

	sections := make([]GoogleChatCardSection, 0)

	// Message
	messageText := event.Message
	if event.Severity == eventv1.EventSeverityError {
		messageText = fmt.Sprintf("<font color=\"#ff0000\">%s</font>", event.Message)
	}
	sections = append(sections, GoogleChatCardSection{
		Widgets: []GoogleChatCardWidget{
			{
				TextParagraph: &GoogleChatCardWidgetTextParagraph{
					Text: messageText,
				},
			},
		},
	})

	// Meta-Data
	if len(event.Metadata) > 0 {
		kvfields := make([]GoogleChatCardWidget, 0, len(event.Metadata))
		for k, v := range event.Metadata {
			kvfields = append(kvfields, GoogleChatCardWidget{
				KeyValue: &GoogleChatCardWidgetKeyValue{
					TopLabel:         k,
					Content:          v,
					ContentMultiLine: false,
				},
			})
		}
		sections = append(sections, GoogleChatCardSection{
			Widgets: kvfields,
		})
	}

	card := GoogleChatCard{
		Header:   header,
		Sections: sections,
	}

	payload := GoogleChatPayload{
		Cards: []GoogleChatCard{card},
	}

	err := postMessage(ctx, s.URL, s.ProxyURL, nil, payload)
	if err != nil {
		return fmt.Errorf("postMessage failed: %w", err)
	}

	return nil
}
