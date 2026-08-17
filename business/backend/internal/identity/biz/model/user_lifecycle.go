package model

import "errors"

var ErrLastActiveOperator = errors.New("identity: the last active platform operator is protected")

func ValidateSubjectTransition(current, target Status) error {
	if current == target && (target == StatusActive || target == StatusDisabled) {
		return nil
	}
	if current == StatusActive && target == StatusDisabled {
		return nil
	}
	if current == StatusDisabled && target == StatusActive {
		return nil
	}
	return ErrConflict
}
