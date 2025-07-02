package notifier

import (
	"context"
	"crypto/sha1"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	eventv1 "github.com/fluxcd/pkg/apis/event/v1beta1"
	"github.com/hashicorp/go-retryablehttp"
)

type Matrix struct {
	Token     string
	URL       string
	RoomId    string
	TLSConfig *tls.Config
}

type MatrixPayload struct {
	Body    string `json:"body"`
	MsgType string `json:"msgtype"`
}

func NewMatrix(serverURL, token, roomId string, tlsConfig *tls.Config) (*Matrix, error) {
	_, err := url.ParseRequestURI(serverURL)
	if err != nil {
		return nil, fmt.Errorf("invalid Matrix homeserver URL %s: '%w'", serverURL, err)
	}

	return &Matrix{
		URL:       serverURL,
		RoomId:    roomId,
		Token:     token,
		TLSConfig: tlsConfig,
	}, nil
}

func (m *Matrix) Post(ctx context.Context, event eventv1.Event) error {
	txId, err := sha1sum(event)
	if err != nil {
		return fmt.Errorf("unable to generate unique tx id: %s", err)
	}
	fullURL := fmt.Sprintf("%s/_matrix/client/r0/rooms/%s/send/m.room.message/%s",
		m.URL, m.RoomId, txId)

	emoji := "💫"
	if event.Severity == eventv1.EventSeverityError {
		emoji = "🚨"
	}
	var metadata string
	for k, v := range event.Metadata {
		metadata = metadata + fmt.Sprintf("- %s: %s\n", k, v)
	}
	heading := fmt.Sprintf("%s %s/%s.%s", emoji, strings.ToLower(event.InvolvedObject.Kind),
		event.InvolvedObject.Name, event.InvolvedObject.Namespace)
	msg := fmt.Sprintf("%s\n%s\n%s", heading, event.Message, metadata)

	payload := MatrixPayload{
		Body:    msg,
		MsgType: "m.text",
	}

	opts := []postOption{
		withRequestModifier(func(req *retryablehttp.Request) {
			req.Method = http.MethodPut
			req.Header.Add("Authorization", "Bearer "+m.Token)
		}),
	}
	if m.TLSConfig != nil {
		opts = append(opts, withTLSConfig(m.TLSConfig))
	}

	if err := postMessage(ctx, fullURL, payload, opts...); err != nil {
		return fmt.Errorf("postMessage failed: %w", err)
	}

	return nil
}

func sha1sum(event eventv1.Event) (string, error) {
	val, err := json.Marshal(event)
	if err != nil {
		return "", err
	}
	digest := sha1.Sum(val)
	return fmt.Sprintf("%x", digest), nil
}
