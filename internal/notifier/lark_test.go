package notifier

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	fuzz "github.com/AdaLogics/go-fuzz-headers"
	eventv1 "github.com/fluxcd/pkg/apis/event/v1beta1"
	"github.com/stretchr/testify/require"
)

func TestLark_Post(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		var payload LarkPayload
		err = json.Unmarshal(b, &payload)
		require.NoError(t, err)

		require.Equal(t, "💫 gitrepository/webapp.gitops-system", payload.Card.Header.Title.Content)
		require.Equal(t, "turquoise", payload.Card.Header.Template)
	}))
	defer ts.Close()

	lark, err := NewLark(ts.URL)
	require.NoError(t, err)

	err = lark.Post(context.TODO(), testEvent())
	require.NoError(t, err)
}

func Fuzz_Lark(f *testing.F) {
	f.Add("", "", "error", []byte{}, []byte{})
	f.Add("", "update", "error", []byte{}, []byte{})

	f.Fuzz(func(t *testing.T,
		urlSuffix, commitStatus, severity string, seed, response []byte) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write(response)
			io.Copy(io.Discard, r.Body)
			r.Body.Close()
		}))
		defer ts.Close()

		lark, err := NewLark(fmt.Sprintf("%s/%s", ts.URL, urlSuffix))
		if err != nil {
			return
		}

		event := eventv1.Event{}
		_ = fuzz.NewConsumer(seed).GenerateStruct(&event)

		if event.Metadata == nil {
			event.Metadata = map[string]string{}
		}

		event.Metadata["commit_status"] = commitStatus
		event.Severity = severity

		_ = lark.Post(context.TODO(), event)
	})
}
