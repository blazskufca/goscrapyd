package assert

import (
	"strings"
	"testing"
)

func Equal[T comparable](t *testing.T, actual, expected T) {
	t.Helper()

	if actual != expected {
		t.Errorf("got: %v; want: %v", actual, expected)
	}
}

func NotEqual[T comparable](t *testing.T, actual, dontWant T) {
	t.Helper()

	if actual == dontWant {
		t.Errorf("got: %v; DON'T want: %v", actual, dontWant)
	}
}

func StringContains(t *testing.T, actual, expectedSubstring string) {
	t.Helper()

	if !strings.Contains(actual, expectedSubstring) {
		t.Errorf("got: %q; expected to contain: %q", actual, expectedSubstring)
	}
}

func StringDoesNotContain(t *testing.T, actual, expectedSubstring string) {
	t.Helper()
	if strings.Contains(actual, expectedSubstring) {
		t.Errorf("got: %q; expected to NOT contain: %q", actual, expectedSubstring)
	}
}

func NilError(t *testing.T, actual error) {
	t.Helper()

	if actual != nil {
		t.Errorf("got: %v; expected: nil", actual)
	}
}
