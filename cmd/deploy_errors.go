package cmd

type KnownLocalUserAndIdError struct{}
type LocalUsernameExistsError struct{}
type LocalUidExistsError struct{}

func (u *KnownLocalUserAndIdError) Error() string {
	return "The Username and UID already exist"
}

func (u *LocalUsernameExistsError) Error() string {
	return "The Username already exists, but does not match the UID. If username exists, the specified uid must match"
}

func (u *LocalUidExistsError) Error() string {
	return "The specified UID already exists, but belongs to a different user. If only Uid exists, the specified username must match"
}
