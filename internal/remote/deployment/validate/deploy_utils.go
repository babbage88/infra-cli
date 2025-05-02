package main

import (
	"flag"
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/user"
	"strings"
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

func parseEnvarsFromStringFlag(strFlag string) map[string]string {
	envVarMap := make(map[string]string)
	envVarsSlice := strings.Split(strFlag, ",")
	for _, value := range envVarsSlice {
		enVar := strings.Split(value, "=")
		envKey := enVar[0]
		envVal := enVar[1]
		slog.Info("Parsed envar", slog.String(envKey, envVal))
		envVarMap[envKey] = envVal

	}
	return envVarMap
}

func main() {
	var validateUser bool
	var installService bool
	var username string
	var uid int64
	var appName string
	var installDir string
	var execBin string
	var systemdDir string
	var systemdServiceUser string
	var envarsFlag string
	// envVarMap := make(map[string]string)
	hostname, _ := os.Hostname()

	flag.BoolVar(&validateUser, "validate-user", true, "Validate a username/UID pair")
	flag.BoolVar(&installService, "enable-systemd", false, "Install Systemd Unit File and start service")

	flag.StringVar(&username, "username", "", "Username to validate")
	flag.Int64Var(&uid, "uid", 8888, "UID to validate")

	flag.StringVar(&appName, "app-name", "", "Application name")
	flag.StringVar(&installDir, "install-dir", "", "Application install dir")
	flag.StringVar(&execBin, "exec-bin", "", "Binary name")
	flag.StringVar(&systemdServiceUser, "svcuser", "", "Systemd unit file directory")
	flag.StringVar(&systemdDir, "systemd-dir", "/etc/systemd", "Systemd unit file directory")
	flag.StringVar(&envarsFlag, "env-vars", "", "Env vars, eg: DATABASE_URL=postgres://dbuser:pass@dbsrv/appdb")
	flag.Parse()
	configureDefaultLogger(slog.LevelDebug)
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
	/*
		if envarsFlag != "" {
			slog.Info("starting envar parsing")
			envVarMap = parseEnvarsFromStringFlag(envarsFlag)
			envVarMap["name"] = appName
		} else {
			envVarMap["name"] = appName
		}
	*/
	if installService {
		err := createSystemdUnitOnLocal(appName, systemdDir, installDir, execBin, systemdServiceUser, envarsFlag)
		if err != nil {
			slog.Error("error creating systemd service file", "error", err.Error())
			os.Exit(1)
		}
		slog.Info("service created")
		os.Exit(0)
	}
}
