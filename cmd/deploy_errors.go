package cmd

type LocalUidExistsError struct{}

func (u *LocalUidExistsError) Error() string {
	return "The specified UID already exists, but belongs to a different user. If only Uid exists, the specified username must match"
}

type KnownLocalUserAndIdError struct{}

func (e *KnownLocalUserAndIdError) Error() string { return "username and UID match an existing user" }

type LocalUsernameExistsError struct{}

func (e *LocalUsernameExistsError) Error() string {
	return "username or UID exists, but they do not match"
}
