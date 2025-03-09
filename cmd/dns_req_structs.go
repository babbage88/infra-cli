package cmd

import (
	"time"

	"github.com/cloudflare/cloudflare-go"
)

type DnsRecordModRequest struct {
	NewOrModifiyRecords map[string]DnsRecord `yaml:"zone_id" json:"zone_id"`
}

type DnsRecord struct {
	ID         string    `yaml:"id" json:"id"`
	ZoneName   string    `yaml:"zone_name" json:"zone_name"`
	Name       string    `yaml:"name" json:"name"`
	Content    string    `yaml:"content" json:"content"`
	Type       string    `yaml:"type" json:"type"`
	TTL        int       `yaml:"ttl" json:"ttl"`
	Proxied    bool      `yaml:"proxied" json:"proxied"`
	Comment    string    `yaml:"comment" json:"comment"`
	Priority   *uint16   `yaml:"priority" json:"priority"`
	Tags       []string  `yaml:"tags" json:"tags"`
	ModifiedOn time.Time `yaml:"modified" json:"modified"`
	CreatedOn  time.Time `yaml:"created" json:"created"`
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
