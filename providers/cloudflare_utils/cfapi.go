package cloudflare_utils

import (
	"context"
	"database/sql"
	"fmt"
	"os"

	"github.com/babbage88/infra-cli/internal/pretty"
	"github.com/cloudflare/cloudflare-go"
)

var logger = pretty.NewCustomLogger(os.Stdout, "DEBUG", 1, "|", true)

type CloudflareCommandUtils struct {
	ZomeId    string          `json:"zoneId"`
	ZoneName  string          `json:"zoneName"`
	EnvFile   string          `json:"envFile"`
	Error     error           `json:"error"`
	ApiClient *cloudflare.API `json:"clouflareApi"`
	DbConn    *sql.DB         `json:"db"`
	UseEnv    bool            `json:"useEnv"`
}

func NewCloudflareCommand(token string, domainName string) *CloudflareCommandUtils {
	cfcmd := &CloudflareCommandUtils{UseEnv: false, ZoneName: domainName}
	cfcmd.NewApiClientFromToken(token)
	if cfcmd.Error == nil {
		cfcmd.ZomeId, cfcmd.Error = cfcmd.ApiClient.ZoneIDByName(domainName)
	}

	return cfcmd
}

func (cf *CloudflareCommandUtils) NewApiClientFromToken(token string) {
	cf.ApiClient, cf.Error = cloudflare.NewWithAPIToken(token)
}

func (cfcmd *CloudflareCommandUtils) ListDNSRecords(params cloudflare.ListDNSRecordsParams) ([]cloudflare.DNSRecord, *cloudflare.ResultInfo) {
	records := []cloudflare.DNSRecord{}
	results := &cloudflare.ResultInfo{}
	records, results, cfcmd.Error = cfcmd.ApiClient.ListDNSRecords(context.Background(), cloudflare.ZoneIdentifier(cfcmd.ZomeId), params)
	return records, results
}

func (cfcmd *CloudflareCommandUtils) GetDnsRecord(recordId string) cloudflare.DNSRecord {
	record := cloudflare.DNSRecord{}
	record, cfcmd.Error = cfcmd.ApiClient.GetDNSRecord(context.Background(), cloudflare.ZoneIdentifier(cfcmd.ZomeId), recordId)
	return record
}

func (cfcmd *CloudflareCommandUtils) DeleteCloudflareRecord(recordId string) {
	cfcmd.Error = cfcmd.ApiClient.DeleteDNSRecord(context.Background(), cloudflare.ZoneIdentifier(cfcmd.ZomeId), recordId)
	if cfcmd.Error == nil {
		msg := fmt.Sprintf("DNS RecordID: %s in Zone: %s has been deleted succesfully", recordId, cfcmd.ZomeId)
		pretty.PrettyLogInfoString(msg)
	}
}

func createOrUpdateCloudflareDnsRecord[T DnsRequestHandler](api cloudflare.API, zoneId string, params T) (cloudflare.DNSRecord, error) {
	record := cloudflare.DNSRecord{}
	var err error = nil
	switch v := any(params).(type) {
	case cloudflare.UpdateDNSRecordParams:
		record, err = api.UpdateDNSRecord(context.Background(), cloudflare.ZoneIdentifier(zoneId), v)
		if err != nil {
			msg := fmt.Sprintf("Error updating DNS record %s in Zone: %s err: %s", v.ID, zoneId, err.Error())
			logger.Error(msg)
			return record, err
		}
		return record, err
	case cloudflare.CreateDNSRecordParams:
		record, err = api.CreateDNSRecord(context.Background(), cloudflare.ZoneIdentifier(zoneId), v)
		if err != nil {
			msg := fmt.Sprintf("Error updating DNS record %s in Zone: %s err: %s", v.Name, zoneId, err.Error())
			logger.Error(msg)
			return record, err
		}
	default:
		err = fmt.Errorf("unsupported DNS record operation %T. Must use cloudflare.UpdateDNSRecordParams or CreateDNSRecordParams", params)
	}
	return record, err
}
