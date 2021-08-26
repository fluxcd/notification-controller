package notifier

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewTelegram(t *testing.T) {
	s, err := NewTelegram("@test", "token")
	require.NoError(t, err)

	url := "telegram://token@telegram?channels=@test&parseMode=markDownv2"
	require.Equal(t, url, s.URL)
}
