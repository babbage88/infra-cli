package ssh

type SshInitializationError struct {
	Message string `json:"message"` // Human readable message for clients
	Code    int    `json:"-"`       // HTTP Status code. We use `-` to skip json marshaling.
	Err     error  `json:"-"`       // The original error. Same reason as above.
}

func SshErrorWrapper(code int, err error, message string) error {
	return SshInitializationError{
		Message: message,
		Code:    code,
		Err:     err,
	}
}

// Implements the errors.Unwrap interface
func (err SshInitializationError) Unwrap() error {
	return err.Err
}

func (err SshInitializationError) Error() string {
	if err.Err != nil {
		return err.Err.Error()
	}
	return err.Message
}

type SftpInitializationError struct {
	Message string `json:"message"` // Human readable message for clients
	Code    int    `json:"-"`       // HTTP Status code. We use `-` to skip json marshaling.
	Err     error  `json:"-"`       // The original error. Same reason as above.
}

func SftpInitErrorWrapper(code int, err error, message string) error {
	return SftpInitializationError{
		Message: message,
		Code:    code,
		Err:     err,
	}
}

// Implements the errors.Unwrap interface
func (err SftpInitializationError) Unwrap() error {
	return err.Err
}

func (err SftpInitializationError) Error() string {
	if err.Err != nil {
		return err.Err.Error()
	}
	return err.Message
}

type SftpTransferError struct {
	Message string `json:"message"` // Human readable message for clients
	Code    int    `json:"-"`       // HTTP Status code. We use `-` to skip json marshaling.
	Err     error  `json:"-"`       // The original error. Same reason as above.
}

func SftpErrorWrapper(code int, err error, message string) error {
	return SftpTransferError{
		Message: message,
		Code:    code,
		Err:     err,
	}
}

// Implements the errors.Unwrap interface
func (err SftpTransferError) Unwrap() error {
	return err.Err
}

func (err SftpTransferError) Error() string {
	if err.Err != nil {
		return err.Err.Error()
	}
	return err.Message
}
