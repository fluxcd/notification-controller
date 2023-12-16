package notifier

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewNtfy(t *testing.T) {
	t.Run("default values", func(t *testing.T) {
		n, err := NewNtfy("https://ntfy.sh", "my-topic", "token", "user", "pass")
		assert.NoError(t, err)
		assert.Equal(t, "https://ntfy.sh", n.ServerURL)
		assert.Equal(t, "my-topic", n.Topic)
		assert.Equal(t, "token", n.Token)
		assert.Equal(t, "user", n.Username)
		assert.Equal(t, "pass", n.Password)
	})

	t.Run("invalid URL", func(t *testing.T) {
		_, err := NewNtfy("not a url", "my-topic", "", "", "")
		assert.Contains(t, err.Error(), "invalid Ntfy server URL")
	})

	t.Run("empty topic", func(t *testing.T) {
		_, err := NewNtfy("https://ntfy.sh", "", "", "", "")
		assert.Equal(t, err.Error(), "ntfy topic cannot be empty")
	})
}

func TestNtfy_Post(t *testing.T) {

	t.Run("success", func(t *testing.T) {
		evt := testEvent()

		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			b, err := io.ReadAll(r.Body)
			require.NoError(t, err)

			var payload NtfyMessage
			err = json.Unmarshal(b, &payload)
			require.NoError(t, err)

			assert.Equal(t, "my-topic", payload.Topic)
			assert.Equal(t, "FluxCD: source-controller", payload.Title)
			assert.Equal(t, []string{NtfyTagInfo}, payload.Tags)
			assert.Equal(t, "message\n\nObject: gitops-system/webapp.GitRepository\n\nMetadata:\ntest: metadata\n", payload.Message)
		}))
		defer ts.Close()

		ntfy, err := NewNtfy(ts.URL, "my-topic", "", "", "")
		require.NoError(t, err)

		err = ntfy.Post(context.Background(), evt)
		require.NoError(t, err)
	})

	t.Run("basic authorization", func(t *testing.T) {
		evt := testEvent()

		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, r.Header.Get("Authorization"), "Basic YmFzaWMtdXNlcjpiYXNpYy1wYXNzd29yZA==")
		}))
		defer ts.Close()

		ntfy, err := NewNtfy(ts.URL, "my-topic", "", "basic-user", "basic-password")
		require.NoError(t, err)

		err = ntfy.Post(context.Background(), evt)
		require.NoError(t, err)
	})

	t.Run("access token", func(t *testing.T) {
		evt := testEvent()

		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, r.Header.Get("Authorization"), "Bearer access-token")
		}))
		defer ts.Close()

		ntfy, err := NewNtfy(ts.URL, "my-topic", "access-token", "", "")
		require.NoError(t, err)

		err = ntfy.Post(context.Background(), evt)
		require.NoError(t, err)
	})
}
