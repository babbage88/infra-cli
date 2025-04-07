package cmd

import (
	"fmt"
	"log"
	"os"
	"os/user"
)

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

// Perform a Username and UID lookup on the local host where infractl command is unexpected.
// Used to verify a proposed service account user either already exists and matches the supplied UID or neither exist.
// If both exist and match, the consumer can skip user creation. If neither exist, they can be safely created.
// Currently only verified for Linux based OSes.
//
// In Windows, UID is a string oR UUID style value.
// Also, the usecases where a service account UID matters are far fewer,
// since AD or LDAP is much more common and containers are much less common.
//
// So for now, I'll leave Windows support way down in the backlog.
//
// But should add a runtime arch check somewhere in the chain.
func lookupLocalUidUnamePair(serviceUser string, serviceUid int64) error {
	uid := fmt.Sprintf("%d", serviceUid)

	usernameLookup, err := user.Lookup(serviceUser)
	usernameExists := false
	usernameMatchesUID := false

	if err != nil {
		if err.Error() != fmt.Sprintf("user: unknown user %s", serviceUser) {
			return fmt.Errorf("unexpected error looking up username: %w", err)
		}
	} else {
		usernameExists = true
		if usernameLookup.Uid == uid {
			usernameMatchesUID = true
		}
	}

	uidLookup, err := user.LookupId(uid)
	uidExists := false
	uidMatchesUsername := false

	if err != nil {
		if err.Error() != fmt.Sprintf("user: unknown userid %s", uid) {
			return fmt.Errorf("unexpected error looking up uid: %w", err)
		}
	} else {
		uidExists = true
		if uidLookup.Username == serviceUser {
			uidMatchesUsername = true
		}
	}

	// Decision logic
	switch {
	case usernameExists && uidExists && usernameMatchesUID && uidMatchesUsername:
		return &KnownLocalUserAndIdError{}
	case usernameExists && !usernameMatchesUID:
		return &LocalUsernameExistsError{}
	case uidExists && !uidMatchesUsername:
		return &LocalUsernameExistsError{}
	default:
		return nil // neither exist, or both exist but not paired
	}
}

// port of the internal/remot/deployment/validate main func not sure i need yet.
func validateSvcUserInput(uname string, userId int64) error {
	hostname, _ := os.Hostname()
	log.Printf("Validating Username: %s and UID: %d on Host: %s", uname, userId, hostname)
	err := lookupLocalUidUnamePair(uname, userId)

	switch err.(type) {
	case *KnownLocalUserAndIdError:
		log.Println("Username and UID both exist and match.")
		return nil
	case *LocalUsernameExistsError:
		log.Println("Username or UID exists, but they do not match.")
		return err
	case nil:
		log.Println("Username and UID are a valid pair; neither currently exist.")
		return nil
	default:
		log.Printf("Unexpected error validating user: %s\n", err)
		return err
	}
}
