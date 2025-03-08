package cloudflare_utils

import (
	"context"
	"fmt"

	"github.com/babbage88/infra-cli/internal/pretty"
	"github.com/cloudflare/cloudflare-go"
)

type DnsRequestHandler interface {
	cloudflare.UpdateDNSRecordParams | cloudflare.CreateDNSRecordParams
}

func NewCloudflareAPIClient(token string) (*cloudflare.API, error) {
	api, err := cloudflare.NewWithAPIToken(token)
	if err != nil {
		msg := fmt.Sprintf("Error initializing lodflaref api client. Verify token. %s", err)
		pretty.PrettyLogErrorString(msg)
		return api, err
	}

	return api, nil
}

func CreateOrUpdateCloudflareDnsRecord[T DnsRequestHandler](token string, zoneId string, params T) (cloudflare.DNSRecord, error) {
	record := cloudflare.DNSRecord{}
	api, err := NewCloudflareAPIClient(token)
	if err != nil {
		return record, err
	}

	switch v := any(params).(type) {
	case cloudflare.UpdateDNSRecordParams:
		record, err = api.UpdateDNSRecord(context.Background(), cloudflare.ZoneIdentifier(zoneId), v)
	case cloudflare.CreateDNSRecordParams:
		record, err = api.CreateDNSRecord(context.Background(), cloudflare.ZoneIdentifier(zoneId), v)
		if err != nil {
			msg := fmt.Sprintf("Error updating dns record %s", err)
			pretty.PrettyLogErrorString(msg)
			return record, err
		}
	default:
		err = fmt.Errorf("unsupported DNS record operation: %T", params)
	}

	return record, err
}
