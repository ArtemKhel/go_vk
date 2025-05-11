package subpub

import "errors"

var (
	ErrClosed       = errors.New("subpub: closed")
	ErrSubjectEmpty = errors.New("subpub: subject cannot be empty")
	ErrHandlerNil   = errors.New("subpub: message handler cannot be nil")
)
