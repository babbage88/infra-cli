package cmd

import (
	"time"

	"github.com/babbage88/infra-cli/internal/pretty"
	"github.com/babbage88/infra-cli/providers/cloudflare_utils"
	"github.com/cloudflare/cloudflare-go"
)

type DnsRecordBatchRequest struct {
	Records []DnsRecord `yaml:"records" json:"records" mapstructure:"records,omitempty"`
	ZoneIDs map[string]string
}

type DnsRecord struct {
	ID         string    `yaml:"id,omitempty" json:"id,omitempty" mapstructure:"id,omitempty"`
	ZoneName   string    `yaml:"zone_name" json:"zone_name" mapstructure:"zone_name"`
	Name       string    `yaml:"name" json:"name" mapstructure:"name"`
	Content    string    `yaml:"content" json:"content" mapstructure:"content"`
	Type       string    `yaml:"type" json:"type" mapstructure:"type"`
	TTL        int       `yaml:"ttl" json:"ttl" mapstructure:"ttl"`
	Proxied    bool      `yaml:"proxied" json:"proxied" mapstructure:"proxied,omitempty"`
	Comment    string    `yaml:"comment" json:"comment" mapstructure:"comment,omitempty"`
	Priority   *uint16   `yaml:"priority" json:"priority" mapstructure:"priority,omitempty"`
	Tags       []string  `yaml:"tags" json:"tags" mapstructure:"tags,omitempty"`
	ModifiedOn time.Time `yaml:"modified" json:"modified" mapstructure:"modified,omitempty"`
	CreatedOn  time.Time `yaml:"created" json:"created" mapstructure:"created,omitempty"`
}

func (cr *DnsRecord) ParseToCloudflareCreateParams() *cloudflare.CreateDNSRecordParams {
	cfreq := &cloudflare.CreateDNSRecordParams{
		Name:       cr.Name,
		Content:    cr.Content,
		TTL:        cr.TTL,
		Type:       cr.ZoneName,
		Proxied:    &cr.Proxied,
		Comment:    cr.Comment,
		Tags:       cr.Tags,
		Priority:   cr.Priority,
		ModifiedOn: cr.ModifiedOn,
		CreatedOn:  cr.CreatedOn,
	}

	return cfreq
}

func (cr *DnsRecord) ParseToCloudflareUpdateParams() *cloudflare.UpdateDNSRecordParams {
	cfreq := &cloudflare.UpdateDNSRecordParams{
		ID:       cr.ID,
		Name:     cr.Name,
		Content:  cr.Content,
		TTL:      cr.TTL,
		Type:     cr.ZoneName,
		Proxied:  &cr.Proxied,
		Comment:  &cr.Comment,
		Tags:     cr.Tags,
		Priority: cr.Priority,
	}

	return cfreq
}

func NewDnsRecordsBatch(apiToken string, records []DnsRecord, zoneIds map[string]string) (*DnsRecordBatchRequest, error) {
	batch := &DnsRecordBatchRequest{Records: records, ZoneIDs: zoneIds}
	err := batch.GetAllZoneIdsForBatch(apiToken)
	if err != nil {
		pretty.PrintErrorf("Error retrieving ZoneIDs for records %s", err.Error())
		return batch, err
	}
	return batch, err
}

func (b *DnsRecordBatchRequest) GetAllZoneIdsForBatch(apiToken string) error {
	if b.ZoneIDs == nil {
		b.ZoneIDs = make(map[string]string)
	}
	for _, record := range b.Records {
		_, exists := b.ZoneIDs[record.ZoneName]
		if !exists {
			zoneId, err := cloudflare_utils.GetCloudFlareZoneIdByDomainName(apiToken, record.ZoneName)
			if err != nil {
				pretty.PrettyErrorLogF("Error retrieving ZoneID for Name %s Records %s", record.Name, err.Error())
				return err
			}
			b.ZoneIDs[record.ZoneName] = zoneId
		}
	}

	return nil
}

func (b *DnsRecordBatchRequest) ProcessDnsBatch(apiToken string) error {
	for _, record := range b.Records {
		if record.ID == "" {
			params := record.ParseToCloudflareCreateParams()
			_, err := cloudflare_utils.CreateOrUpdateCloudflareDnsRecord(apiToken, b.ZoneIDs[record.ZoneName], *params)
			if err != nil {
				pretty.PrintErrorf("Error creating new record. Name %s error: %s", record.Name, err.Error())
				return err
			}
		} else {
			params := record.ParseToCloudflareUpdateParams()
			_, err := cloudflare_utils.CreateOrUpdateCloudflareDnsRecord(apiToken, b.ZoneIDs[record.ZoneName], *params)
			if err != nil {
				pretty.PrintErrorf("Error updating record. Name %s error: %s", record.Name, err.Error())
				return err
			}
		}
	}
	return nil
}
