package vpc

import (
	"errors"
	"testing"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
)

func TestBuildResultError(t *testing.T) {
	t.Run("recorded error is returned", func(t *testing.T) {
		state := new(multistep.BasicStateBag)
		want := errors.New("create image failed")
		state.Put("error", want)
		// An image_id may or may not be present; a recorded error always wins.
		state.Put("image_id", "r006-deadbeef")

		if got := buildResultError(state); got != want {
			t.Fatalf("expected recorded error to be returned, got %v", got)
		}
	})

	t.Run("halt with no image and no recorded error is a failure", func(t *testing.T) {
		// This is the regression: a step halted (e.g. CreateImage failed) without
		// recording an "error", so no image_id exists. The build must still fail
		// rather than reporting a false success with no artifact.
		state := new(multistep.BasicStateBag)

		if err := buildResultError(state); err == nil {
			t.Fatal("expected an error when no image_id was produced, got nil")
		}
	})

	t.Run("image produced and no error succeeds", func(t *testing.T) {
		state := new(multistep.BasicStateBag)
		state.Put("image_id", "r006-deadbeef")

		if err := buildResultError(state); err != nil {
			t.Fatalf("expected success when image_id is present, got %v", err)
		}
	})
}
