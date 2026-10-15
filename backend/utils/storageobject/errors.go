package storageobject

import "errors"

var (
	ErrConflict               = errors.New("storage object already exists")
	ErrConditionalUnsupported = errors.New("storage does not support conditional creation")
	ErrOutcomeUncertain       = errors.New("storage write outcome is uncertain; object requires investigation")
)
