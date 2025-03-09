package cmd

import "time"

type DnsRecordModRequest struct {
	NewOrModifiyRecords map[string]DnsRecord `yaml:"zone_id" json:"zone_id"`
}

type DnsRecord struct {
	ZoneName string    `yaml:"zone_name" json:"zone_name"`
	Name     string    `yaml:"name" json:"name"`
	Type     string    `yaml:"type" json:"type"`
	TTL      int       `yaml:"ttl" json:"ttl"`
	Proxied  bool      `yaml:"proxied" json:"proxied"`
	Comment  string    `yaml:"comment" json:"comment"`
	Tags     []string  `yaml:"tags" json:"tags"`
	Modified time.Time `yaml:"modified" json:"modified"`
	Created  time.Time `yaml:"created" json:"created"`
}
