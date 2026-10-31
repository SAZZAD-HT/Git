package api

import (
	"fmt"
	"testing"

	"anvil/internal/errs"
)

func TestUserMessage(t *testing.T) {
	cases := []struct {
		err  error
		kind error
		want string
	}{
		{fmt.Errorf("invalid username or password: %w", errs.Unauthenticated), errs.Unauthenticated, "Invalid username or password"},
		{errs.NotFound, errs.NotFound, "Not found"},
		{fmt.Errorf("user bob: %w", errs.NotFound), errs.NotFound, "User bob"},
	}
	for _, c := range cases {
		if got := userMessage(c.err, c.kind); got != c.want {
			t.Errorf("%v -> %q, want %q", c.err, got, c.want)
		}
	}
}
