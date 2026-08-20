package reconcile

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestThreePassUpdate_dependentAbsent(t *testing.T) {
	var calls []string
	err := ThreePassUpdate(
		false,
		func() error { calls = append(calls, "parentReconcile"); return nil },
		func() error { calls = append(calls, "parentCreateOrUpdate"); return nil },
		func() error { calls = append(calls, "dependentReconcile"); return nil },
		func() error { calls = append(calls, "parentDeleteRemoved"); return nil },
	)

	assert.NoError(t, err)
	assert.Equal(t, []string{"parentReconcile", "dependentReconcile"}, calls)
}

func TestThreePassUpdate_dependentPresent(t *testing.T) {
	var calls []string
	err := ThreePassUpdate(
		true,
		func() error { calls = append(calls, "parentReconcile"); return nil },
		func() error { calls = append(calls, "parentCreateOrUpdate"); return nil },
		func() error { calls = append(calls, "dependentReconcile"); return nil },
		func() error { calls = append(calls, "parentDeleteRemoved"); return nil },
	)

	assert.NoError(t, err)
	assert.Equal(t, []string{"parentCreateOrUpdate", "dependentReconcile", "parentDeleteRemoved"}, calls)
}

func TestThreePassUpdate_stopsOnFirstError(t *testing.T) {
	t.Run("dependent absent, parentReconcile fails", func(t *testing.T) {
		var calls []string
		wantErr := errors.New("boom")
		err := ThreePassUpdate(
			false,
			func() error { calls = append(calls, "parentReconcile"); return wantErr },
			func() error { calls = append(calls, "parentCreateOrUpdate"); return nil },
			func() error { calls = append(calls, "dependentReconcile"); return nil },
			func() error { calls = append(calls, "parentDeleteRemoved"); return nil },
		)

		assert.Equal(t, wantErr, err)
		assert.Equal(t, []string{"parentReconcile"}, calls)
	})

	t.Run("dependent present, dependentReconcile fails before delete", func(t *testing.T) {
		var calls []string
		wantErr := errors.New("boom")
		err := ThreePassUpdate(
			true,
			func() error { calls = append(calls, "parentReconcile"); return nil },
			func() error { calls = append(calls, "parentCreateOrUpdate"); return nil },
			func() error { calls = append(calls, "dependentReconcile"); return wantErr },
			func() error { calls = append(calls, "parentDeleteRemoved"); return nil },
		)

		assert.Equal(t, wantErr, err)
		assert.Equal(t, []string{"parentCreateOrUpdate", "dependentReconcile"}, calls)
	})
}
