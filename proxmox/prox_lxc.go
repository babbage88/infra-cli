package proxmox

import (
	"bytes"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
)

func CreateLXCContainer(host, token, node string, params map[string]string) error {
	apiURL := fmt.Sprintf("%s/api2/json/nodes/%s/lxc", host, node)

	form := url.Values{}
	for k, v := range params {
		form.Set(k, v)
	}

	req, err := http.NewRequest("POST", apiURL, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "PVEAPIToken="+token)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var buf bytes.Buffer
	buf.ReadFrom(resp.Body)

	if resp.StatusCode != 200 {
		log.Printf("Proxmox API error: %s\nBody: %s", resp.Status, buf.String())
		return fmt.Errorf("Proxmox API error: %s", resp.Status)
	}

	log.Println("Container created successfully")
	return nil
}
