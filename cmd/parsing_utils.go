package cmd

import (
	"log"
	"strings"
)

func parseCmdStringToSlice(cmdString string) []string {
	cmdSlice := strings.Fields(cmdString)
	//retVal := make([]string, len(cmdSlice))

	//retVal = append(retVal, cmdSlice...)

	return cmdSlice
}

func parseBaseCommand(cmdString string) string {
	cmdSlice := parseCmdStringToSlice(cmdString)
	switch len(cmdSlice) {
	case 0:
		log.Printf("Command string is empty\n")
		return cmdString
	default:
		log.Printf("Parsed Base Command: %s\n", cmdSlice[0])
		return cmdSlice[0]
	}
}

func parseCmdStringArgsToSlice(cmdString string) []string {
	cmdSlice := parseCmdStringToSlice(cmdString)
	switch len(cmdSlice) {
	case 0:
		log.Printf("Command string is empty\n")
		return nil
	default:
		log.Printf("Parsed Command args: %s\n", cmdSlice[1:])
		return cmdSlice[1:]
	}
}

func parseCmdStringToMap(cmdString string) map[string][]string {
	retVal := make(map[string][]string)
	cmdSlice := parseCmdStringToSlice(cmdString)
	switch len(cmdSlice) {
	case 0:
		log.Printf("Command string is empty")
		return nil
	case 1:
		log.Printf("Command string hass no args")
		retVal[cmdSlice[0]] = nil
	default:
		key := cmdSlice[0]
		value := cmdSlice[1:]
		retVal[key] = value
	}

	return retVal

}
