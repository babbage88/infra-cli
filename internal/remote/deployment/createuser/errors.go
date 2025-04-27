package main

// Typed errors for various situations
type KnownUserAndUidExistsError struct{}

func (e *KnownUserAndUidExistsError) Error() string {
	return "username and UID match an existing user"
}

type UsernameAndUidMismatchError struct{}

func (e *UsernameAndUidMismatchError) Error() string {
	return "username and UID exist but do not match"
}

type UsernameExistsError struct{}

func (e *UsernameExistsError) Error() string {
	return "username exists but does not match the specified UID"
}

type UidExistsError struct{}

func (e *UidExistsError) Error() string {
	return "UID exists but does not match the specified username"
}
