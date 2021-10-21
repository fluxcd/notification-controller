package server

import (
	"errors"
	"testing"
)

func TestRedactTokenFromError(t *testing.T) {
	tests := []struct {
		name           string
		token          string
		originalErrStr string
		expectedErrStr string
	}{
		{
			name:           "no token",
			token:          "8h0387hdyehbwwa45",
			originalErrStr: "Cannot post to github",
			expectedErrStr: "Cannot post to github",
		},
		{
			name:           "empty token",
			token:          "",
			originalErrStr: "Cannot post to github",
			expectedErrStr: "Cannot post to github",
		},
		{
			name:           "exact token",
			token:          "8h0387hdyehbwwa45",
			originalErrStr: "Cannot post to github with token 8h0387hdyehbwwa45",
			expectedErrStr: "Cannot post to github with token *****",
		},
		{
			name:           "non-exact token",
			token:          "8h0387hdyehbwwa45",
			originalErrStr: `Cannot post to github with token 8h0387hdyehbwwa45\\n`,
			expectedErrStr: `Cannot post to github with token *****\\n`,
		},
		{
			name:           "extra text in front token",
			token:          "8h0387hdyehbwwa45",
			originalErrStr: `Cannot post to github with token metoo8h0387hdyehbwwa45\\n`,
			expectedErrStr: `Cannot post to github with token metoo*****\\n`,
		},
	}

	for _, tt := range tests {
		err := redactTokenFromError(errors.New(tt.originalErrStr), tt.token)
		if err == nil {
			t.Fatalf("error shouldn't be nil")
		}

		if err.Error() != tt.expectedErrStr {
			t.Errorf("expected error string '%s' but got '%s'",
				tt.expectedErrStr, err)
		}
	}
}
