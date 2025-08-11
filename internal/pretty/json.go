package pretty

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/babbage88/infra-cli/internal/type_helper"
)

const (
	jsonColorReset  = "\033[0m"
	jsonColorCyan   = "\033[38;5;51m"
	jsonColorGreen  = "\033[38;5;49m"
	jsonColorWhite  = "\033[1;97m"
	jsonColorOrange = "\033[38;5;11m" // 256-color orange
)

type VMConfigTyped struct {
	Name   string `json:"name"`
	CPUs   int    `json:"cpus"`
	Memory int    `json:"memory"`
}

func indentStr(n int) string {
	return strings.Repeat("  ", n) // two spaces per indent
}

func PrintColoredJSON(v interface{}, indent int) {
	switch val := v.(type) {
	case map[string]interface{}:
		fmt.Println("{")
		i := 0
		for k, v2 := range val {
			fmt.Printf("%s%s\"%s\"%s: ",
				indentStr(indent+1),
				jsonColorCyan, k, jsonColorReset,
			)
			PrintColoredJSON(v2, indent+1)
			i++
			if i < len(val) {
				fmt.Print(",")
			}
			fmt.Println()
		}
		fmt.Printf("%s}", indentStr(indent))

	case []interface{}:
		fmt.Println("[")
		for i, v2 := range val {
			fmt.Printf("%s", indentStr(indent+1))
			PrintColoredJSON(v2, indent+1)
			if i < len(val)-1 {
				fmt.Print(",")
			}
			fmt.Println()
		}
		fmt.Printf("%s]", indentStr(indent))

	case string:
		if type_helper.IsNumber(val) {
			fmt.Printf("%s\"%s\"%s", jsonColorOrange, val, jsonColorReset)
		} else {
			fmt.Printf("%s\"%s\"%s", jsonColorGreen, val, jsonColorReset)
		}

	case float64:
		// JSON numbers unmarshal as float64
		if reflect.TypeOf(val).Kind() == reflect.Float64 && val == float64(int(val)) {
			fmt.Printf("%s%d%s", jsonColorOrange, int(val), jsonColorReset)
		} else {
			fmt.Printf("%s%f%s", jsonColorOrange, val, jsonColorReset)
		}

	case bool:
		fmt.Printf("%s%t%s", jsonColorWhite, val, jsonColorReset)

	case nil:
		fmt.Print("null")

	default:
		fmt.Printf("%v", val)
	}
}
