package prefix

import "errors"

var (
	OutLenError        = errors.New("OutLenError")
	SentinelCatchError = errors.New("SentinelCatchError")
	NotMatchingError   = errors.New("NotMatchingError")
	NotExistsError     = errors.New("NotExistsError")
	ClosedError        = errors.New("ClosedError")
	CreateError        = errors.New("CreateError")
	ParseError         = errors.New("ParseError")
)
