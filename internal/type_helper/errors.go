package type_helper

import (
	"errors"
	"fmt"
)

// Int64OverflowError represents an overflow error when converting int64 to int.
type Int64OverflowError struct {
	Value int64
	Msg   string
	Err   error
}

// Error implements the error interface.
func (e *Int64OverflowError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %d: %v", e.Msg, e.Value, e.Err)
	}
	return fmt.Sprintf("%s: %d", e.Msg, e.Value)
}

// Unwrap allows errors.Is / errors.As to access the wrapped error.
func (e *Int64OverflowError) Unwrap() error {
	return e.Err
}

// Wrap attaches another error to the Int64OverflowError.
func (e *Int64OverflowError) Wrap(err error) *Int64OverflowError {
	e.Err = err
	return e
}

// ErrInt64FromStringParsing is return when an int64 cannot be parsed from the given string
var ErrInt64FromStringParsing error = errors.New(Int64ParsingErrMsgBase)

// ErrInt32FromStringParsing is return when an int32 cannot be parsed from the given string
var ErrInt32FromStringParsing error = errors.New(Int32ParsingErrMsgBase)
