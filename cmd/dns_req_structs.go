package cmd

import (
	"time"

	"github.com/cloudflare/cloudflare-go"
)

type DnsRecordBatchRequest struct {
	Records []DnsRecord `yaml:"records" json:"records" mapstructure:"records,omitempty"`
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
