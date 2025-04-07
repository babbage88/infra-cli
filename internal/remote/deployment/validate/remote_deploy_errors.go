package main

type RemoteUidExistsError struct{}

func (u *RemoteUidExistsError) Error() string {
	return "The specified UID already exists, but belongs to a different user. If only Uid exists, the specified username must match"
}
