package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/user"
)

func ValidateRemoteUidUnamePair(serviceUser string, serviceUid int64) error {
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
		return &KnownRemoteUserAndIdError{}
	case usernameExists && !usernameMatchesUID:
		return &RemoteUsernameExistsError{}
	case uidExists && !uidMatchesUsername:
		return &RemoteUsernameExistsError{}
	default:
		return nil // neither exist, or both exist but not paired
	}
}

func main() {
	var validateUser bool
	var username string
	var uid int64
	hostname, _ := os.Hostname()

	flag.BoolVar(&validateUser, "validate-user", false, "Validate a username/UID pair")
	flag.StringVar(&username, "username", "", "Username to validate")
	flag.Int64Var(&uid, "uid", 8888, "UID to validate")
	flag.Parse()

	if validateUser {
		log.Printf("Validating Username: %s and UID: %d on Host: %s", username, uid, hostname)
		err := ValidateRemoteUidUnamePair(username, uid)

		switch err.(type) {
		case *KnownRemoteUserAndIdError:
			log.Println("Username and UID both exist and match.")
			os.Exit(0)
		case *RemoteUsernameExistsError:
			log.Println("Username or UID exists, but they do not match.")
			os.Exit(1)
		case nil:
			log.Println("Username and UID are a valid pair; neither currently exist.")
			os.Exit(0)
		default:
			log.Printf("Unexpected error validating user: %s\n", err)
			os.Exit(3)
		}
	}
}
