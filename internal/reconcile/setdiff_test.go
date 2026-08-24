package reconcile

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	fastly "github.com/fastly/go-fastly/v17/fastly"
)

func TestDiffSet(t *testing.T) {
	t.Run("adds missing and removes extra", func(t *testing.T) {
		var added, removed []string

		err := DiffSet(
			[]string{"a", "b"},
			[]string{"b", "c"},
			func(k string) error { added = append(added, k); return nil },
			func(k string) error { removed = append(removed, k); return nil },
		)

		assert.NoError(t, err)
		assert.Equal(t, []string{"c"}, added)
		assert.Equal(t, []string{"a"}, removed)
	})

	t.Run("no-op when already equal", func(t *testing.T) {
		var calls int

		err := DiffSet(
			[]string{"a", "b"},
			[]string{"b", "a"},
			func(_ string) error { calls++; return nil },
			func(_ string) error { calls++; return nil },
		)

		assert.NoError(t, err)
		assert.Zero(t, calls)
	})

	t.Run("tolerates not-found on remove", func(t *testing.T) {
		err := DiffSet(
			[]string{"a"},
			nil,
			func(_ string) error { return nil },
			func(_ string) error { return &fastly.HTTPError{StatusCode: 404} },
		)

		assert.NoError(t, err)
	})

	t.Run("propagates remove error", func(t *testing.T) {
		err := DiffSet(
			[]string{"a"},
			nil,
			func(_ string) error { return nil },
			func(_ string) error { return errors.New("remove failed") },
		)

		assert.EqualError(t, err, "remove failed")
	})

	t.Run("propagates add error", func(t *testing.T) {
		err := DiffSet(
			nil,
			[]string{"a"},
			func(_ string) error { return errors.New("add failed") },
			func(_ string) error { return nil },
		)

		assert.EqualError(t, err, "add failed")
	})
}
