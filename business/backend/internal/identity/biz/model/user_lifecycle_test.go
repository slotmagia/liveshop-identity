package model

import (
	"errors"
	"testing"
)

func TestValidateSubjectTransition(t *testing.T) {
	for _, test := range []struct {
		current, target Status
		allowed         bool
	}{
		{StatusActive, StatusDisabled, true},
		{StatusDisabled, StatusActive, true},
		{StatusActive, StatusActive, true},
		{StatusDisabled, StatusDisabled, true},
		{StatusClosed, StatusActive, false},
		{StatusActive, StatusClosed, false},
	} {
		err := ValidateSubjectTransition(test.current, test.target)
		if test.allowed && err != nil {
			t.Fatalf("%s -> %s: %v", test.current, test.target, err)
		}
		if !test.allowed && !errors.Is(err, ErrConflict) {
			t.Fatalf("%s -> %s must conflict, got %v", test.current, test.target, err)
		}
	}
}
