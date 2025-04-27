package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/user"
)

// AddUserWithUid adds a new user with a specific UID and checks for conflicts.
func AddUserWithUid(serviceUser string, serviceUid int64) error {
	uid := fmt.Sprintf("%d", serviceUid)

	// Check if the username already exists
	usernameLookup, err := user.Lookup(serviceUser)
	usernameExists := false
	if err == nil {
		usernameExists = true
	}

	// Check if the UID already exists
	_, err = user.LookupId(uid)
	uidExists := false
	if err == nil {
		uidExists = true
	}

	// Decision logic
	switch {
	case usernameExists && uidExists:
		if usernameLookup.Uid == uid {
			// The user already exists with the correct UID
			return &KnownUserAndUidExistsError{}
		}
		// Username and UID exist but do not match
		return &UsernameAndUidMismatchError{}
	case usernameExists && !uidExists:
		// Username exists but no user with the given UID
		return &UsernameExistsError{}
	case !usernameExists && uidExists:
		// UID exists but no user with the given username
		return &UidExistsError{}
	default:
		// Neither the username nor the UID exist, proceed with adding the user
		if err := createUser(serviceUser, serviceUid); err != nil {
			return fmt.Errorf("error creating user: %w", err)
		}
		return nil
	}
}

// createUser creates a new user with the specified username and UID.
func createUser(username string, uid int64) error {
	// Attempt to create the user using a system call, e.g., useradd or similar (platform-dependent)
	cmd := exec.Command("useradd", "-u", fmt.Sprintf("%d", uid), username)
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to create user %s with UID %d: %w", username, uid, err)
	}
	return nil
}

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

// Command-line interface for adding a user with a specific UID
func main() {
	var username string
	var uid int64

	flag.StringVar(&username, "username", "", "Username to validate")
	flag.Int64Var(&uid, "uid", 8888, "UID to validate")

	// Parse the flags
	flag.Parse()

	// Check if required flags are provided
	if *&username == "" || *&uid == 0 {
		fmt.Println("Error: Both username and uid are required")
		flag.Usage()
		os.Exit(1)
	}

	// Validate and add the user
	err := AddUserWithUid(*&username, *&uid)
	if err != nil {
		log.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	log.Println("User added successfully.")
}
