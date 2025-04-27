package main

import (
	"flag"
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/exec"
	"os/user"
)

const (
	sudoCmd           string = "sudo"
	useraddCmd        string = "useradd"
	groupaddCmd       string = "groupadd"
	usermodCmd        string = "usermod"
	sudoersDirectory  string = "/etc/sudoers.d"
	sudoersFilePrefix string = "custom-"
	useraddUidFlag    string = "-u"
	useraddGidFlag    string = "-g"
	groupaddGidFlag   string = "-g"
	rootUid           int64  = 0
	rootUidStr        string = "0"
	rootGid           int64  = 0
	rootGidStr        string = "0"
	rootUsername      string = "root"
	sudoGroupname     string = "wheel"
	wheelGroupname    string = "wheel"
	adminGroupname    string = "admin"
)

func buildCommandArgs(useSudo bool, newUid int64, newUsername string, newGid int64) (string, []string) {
	newUidStr := fmt.Sprintf("%d", newUid)
	newGidStr := fmt.Sprintf("%d", newGid)
	var cmdBase string
	var cmdArgs []string
	if useSudo {
		cmdBase = sudoCmd
		cmdArgs = []string{useraddCmd, useraddUidFlag, newUidStr, useraddGidFlag, newGidStr, newUsername}
		return cmdBase, cmdArgs
	} else {
		cmdBase = useraddCmd
		cmdArgs = []string{useraddUidFlag, newUidStr, useraddGidFlag, newGidStr, newUsername}
		return cmdBase, cmdArgs
	}

}

// AddUserWithUid adds a new user with a specific UID and checks for conflicts.
func AddUserWithUid(serviceUser string, serviceUid int64, serviceGid int64) error {
	var crtCmd string
	var cmdArgs []string
	var canSudo bool
	serviceUidStr := fmt.Sprintf("%d", serviceUid)

	currentUser, err := getCurrentUserInfo()
	if err != nil {
		slog.Error("unable to retrieve current user info", "error", err)
	}
	isRoot := currentUser.Uid == rootUidStr || currentUser.Username == rootUsername || currentUser.Gid == rootGidStr
	canSudo = checkUserHasSudo(currentUser)

	// Check if the username already exists
	usernameLookup, err := user.Lookup(serviceUser)
	usernameExists := false
	if err == nil {
		usernameExists = true
	}

	// Check if the UID already exists
	_, err = user.LookupId(serviceUidStr)
	uidExists := false
	if err == nil {
		uidExists = true
	}

	if isRoot {
		crtCmd, cmdArgs = buildCommandArgs(false, serviceUid, serviceUser, serviceGid)
	} else {
		crtCmd, cmdArgs = buildCommandArgs(canSudo, serviceUid, serviceUser, serviceGid)
	}

	// Decision logic
	switch {
	case usernameExists && uidExists:
		if usernameLookup.Uid == serviceUidStr {
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
		if err := createUser(serviceUser, serviceUid, serviceGid); err != nil {
			slog.Error("Error running the createUser, checking if current user has sudo.")
			if canSudo {
				slog.Info("The current user seems to have sudo privileges, attempting useradd using sudo",
					slog.String("CurrentUsername", currentUser.Username), slog.String("currentUid", currentUser.Uid))
				cmd := exec.Command(crtCmd, cmdArgs...)
				fmt.Println("Running command: ", crtCmd, "args: ", cmdArgs)
				if err := cmd.Run(); err != nil {
					slog.Error("Error running elevater command. Please verify user is properly configured in sudoers file or run as root",
						slog.String("currentuser.Username", currentUser.Username), slog.String("currentuser.Uid", currentUser.Uid))
					return fmt.Errorf("error creating user using sudo: %w", err)

				}
			}

			return fmt.Errorf("error creating user: %w", err)
		}
		slog.Info("User created successfully",
			slog.String("newUsername", serviceUser), slog.Int64("newUid", serviceUid),
			slog.String("createdBy", currentUser.Username),
			slog.String("createdByUid", currentUser.Uid))
		return nil
	}
}

// createUser creates a new user with the specified username and UID.
func createUser(username string, uid int64, gid int64) error {
	usrAddCommand, usrAddArgs := buildCommandArgs(false, uid, username, gid)
	cmd := exec.Command(usrAddCommand, usrAddArgs...)
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to create user %s with UID %d: %w", username, uid, err)
	}
	return nil
}

func getCurrentUserInfo() (*user.User, error) {
	currentUser, err := user.Current()
	if err != nil {
		slog.Error("Failed to get current user", "error", err.Error())
		return nil, err
	}
	return currentUser, nil
}

func checkUserHasSudo(u *user.User) bool {
	groupIDs, err := u.GroupIds()
	if err != nil {
		log.Fatalf("Failed to get user's group IDs: %v", err)
	}

	var groups []string
	for _, gid := range groupIDs {
		group, err := user.LookupGroupId(gid)
		if err == nil {
			groups = append(groups, group.Name)
		}
	}

	hasGroupSudo := false
	for _, group := range groups {
		if group == sudoGroupname || group == wheelGroupname || group == adminGroupname {
			hasGroupSudo = true
			break
		}
	}

	if hasGroupSudo {
		fmt.Println("User likely has sudo privileges (belongs to sudo/wheel/admin group).")
		return true
	}

	// If not obvious, parse sudoers files
	hasSudoersSudo, err := parseSudoersFiles(u.Username, groups)
	if err != nil {
		fmt.Println("Warning: Error parsing sudoers files:", err)
	}
	if hasSudoersSudo {
		fmt.Println("User has sudo privileges according to sudoers file entries.")
		return true
	}

	fmt.Println("User does NOT appear to have sudo privileges.")
	return false
}

// Command-line interface for adding a user with a specific UID
func main() {
	var username string
	var uid int64
	var gid int64
	var checkSudo bool

	flag.StringVar(&username, "username", "", "Username to create")
	flag.Int64Var(&uid, "uid", 8888, "UID for the new user")
	flag.Int64Var(&gid, "gid", uid, "GID for the new user")
	flag.BoolVar(&checkSudo, "check-sudo", false, "Check if the current user has sudo privileges")

	// Parse the flags
	flag.Parse()

	if checkSudo {
		curUser, err := getCurrentUserInfo()
		if err != nil {
			log.Fatalf("Error retrieving info for the current user")
		}
		canSudo := checkUserHasSudo(curUser)
		if canSudo {
			fmt.Printf("The current username %s uid: %s appears to have sudo privileges\n", curUser.Username, curUser.Uid)
			os.Exit(0)
		}

	}

	// Check if required flags are provided
	if *&username == "" || *&uid == 0 {
		fmt.Println("Error: Both username and uid are required")
		flag.Usage()
		os.Exit(1)
	}

	// Validate and add the user
	err := AddUserWithUid(*&username, *&uid, *&gid)
	if err != nil {
		log.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	slog.Info("User added successfully.", slog.String("username", username), slog.Int64("uid", uid), slog.Int64("gid", gid))
}
