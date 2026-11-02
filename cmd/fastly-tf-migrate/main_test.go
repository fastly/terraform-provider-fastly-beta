package main

import (
	"errors"
	"fmt"
	"testing"
)

type testExitError struct {
	code int
}

func (e testExitError) Error() string {
	return "test exit error"
}

func (e testExitError) ExitCode() int {
	return e.code
}

func TestExitCode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want int
	}{
		{
			name: "plain error",
			err:  errors.New("boom"),
			want: 1,
		},
		{
			name: "typed exit error",
			err:  testExitError{code: 42},
			want: 42,
		},
		{
			name: "wrapped typed exit error",
			err:  fmt.Errorf("wrapped: %w", testExitError{code: 7}),
			want: 7,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			if got := exitCode(test.err); got != test.want {
				t.Fatalf("exitCode() = %d, want %d", got, test.want)
			}
		})
	}
}
