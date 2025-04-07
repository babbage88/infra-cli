package main

type RemoteUidExistsError struct{}

func (u *RemoteUidExistsError) Error() string {
	return "The specified UID already exists, but belongs to a different user. If only Uid exists, the specified username must match"
}

type KnownRemoteUserAndIdError struct{}

func (e *KnownRemoteUserAndIdError) Error() string { return "username and UID match an existing user" }

type RemoteUsernameExistsError struct{}

func (e *RemoteUsernameExistsError) Error() string {
	return "username or UID exists, but they do not match"
}
