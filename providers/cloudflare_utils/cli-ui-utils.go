package cloudflare_utils

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"text/tabwriter"

	"github.com/babbage88/infra-cli/internal/pretty"
	"github.com/cloudflare/cloudflare-go"
)

func printJson(data any) {
	response, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		slog.Error("error marshaling response.", slog.String("error", err.Error()))
	}
	fmt.Printf("%s\n", string(response))
	fmt.Println()
}

func (cfcmd *CloudflareCommandUtils) PrintCommandResultAsJson(result any) string {
	fmt.Println()
	response, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		cfcmd.Error = err
		msg := fmt.Sprintf("error marshaling result into json. error: %s", err.Error())
		pretty.PrettyLogErrorString(msg)
	}
	recordsJson := fmt.Sprintf("%s\n", (string(response)))
	pretty.Print(recordsJson)

	return recordsJson
}

func (cfcmd *CloudflareCommandUtils) PrintZoneIdTable() error {
	var colorInt int32 = 97
	coloStartSting := fmt.Sprintf("\x1b[1;%dm", colorInt)
	colorEndString := "\x1b[0m"
	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(tw, "%sZoneName\tZoneID%s\n", coloStartSting, colorEndString)
	fmt.Fprintf(tw, "%s--------\t------%s\n", coloStartSting, colorEndString)
	fmt.Fprintf(tw, "%s%s\t%s%s\n", coloStartSting, cfcmd.ZoneName, cfcmd.ZomeId, colorEndString)
	err := tw.Flush()
	return err
}

func (cfcmd *CloudflareCommandUtils) PrintDnsRecordsTable(records []cloudflare.DNSRecord) {
	var colorInt int32 = 97
	tw := tabwriter.NewWriter(os.Stdout, 2, 0, 1, ' ', 0)
	fmt.Fprintf(tw, "\x1b[1;%dm%s\t%s\t%s\t%s\t%s\t%s\t%s\x1b[0m\n", colorInt, "ID", "Name", "Content", "Type", "CreatedOn", "ModifiedOn", "Comment")
	fmt.Fprintf(tw, "\x1b[1;%dm--\t----\t-------\t----\t---------\t----------\t-------\x1b[0m\n", colorInt)
	for _, v := range records {
		switch v.Type {
		case "A":
			colorInt = int32(96)
		case "CNAME":
			colorInt = int32(92)
		default:
			colorInt = int32(97)
		}
		fmt.Fprintf(tw, "\x1b[1;%dm%s\t%s\t%s\t%s\t%s\t%s\t%s\x1b[0m\n", colorInt, v.ID, v.Name, v.Content, v.Type, pretty.DateTimeSting(v.CreatedOn), pretty.DateTimeSting(v.ModifiedOn), v.Comment)
	}
	tw.Flush()
	fmt.Printf("\x1b[1;%dm\nFound %d records in ZoneID: %s Name: %s\x1b[0m\n", colorInt, len(records), cfcmd.ZomeId, cfcmd.ZoneName)
}
