package proxmox

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// CreateLxcContainer sends a POST request to create an LXC container.
func CreateLxcContainer(auth Auth, container LxcContainer) error {
	// Build the endpoint
	apiURL := fmt.Sprintf("%s/api2/json/nodes/%s/lxc", auth.Host, container.Node)

	// Build form data
	data := url.Values{}
	data.Set("vmid", fmt.Sprintf("%d", container.VmId))
	data.Set("hostname", container.Hostname)
	data.Set("password", container.Password)
	data.Set("ostemplate", container.OsTemplate)
	data.Set("storage", container.Storage)
	data.Set("rootfs", container.RootFsSize)
	data.Set("memory", fmt.Sprintf("%d", container.Memory))
	data.Set("swap", fmt.Sprintf("%d", container.Swap))
	data.Set("cores", fmt.Sprintf("%d", container.Cores))
	data.Set("cpulimit", fmt.Sprintf("%d", container.CpuLimit))
	data.Set("cpuunits", fmt.Sprintf("%d", container.CpuUnits))
	data.Set("net0", container.Net0)

	if container.Nameserver != "" {
		data.Set("nameserver", container.Nameserver)
	}
	if container.Searchdomain != "" {
		data.Set("searchdomain", container.Searchdomain)
	}
	if container.Pool != "" {
		data.Set("pool", container.Pool)
	}
	if container.Description != "" {
		data.Set("description", container.Description)
	}
	data.Set("unprivileged", fmt.Sprintf("%d", container.Unprivileged))
	data.Set("start", fmt.Sprintf("%d", container.Start))
	data.Set("autostart", fmt.Sprintf("%d", container.Autostart))
	data.Set("bwlimit", fmt.Sprintf("%d", container.BwLimit))
	data.Set("arch", container.Arch)
	data.Set("cmode", container.Cmode)
	data.Set("console", fmt.Sprintf("%d", container.Console))
	data.Set("debug", fmt.Sprintf("%d", container.Debug))
	if container.Features != "" {
		data.Set("features", container.Features)
	}
	if container.Startup != "" {
		data.Set("startup", container.Startup)
	}
	if container.Tags != "" {
		data.Set("tags", container.Tags)
	}
	if container.SshPublicKeys != "" {
		data.Set("ssh-public-keys", container.SshPublicKeys)
	}

	// Build and send the request
	req, err := http.NewRequest("POST", apiURL, strings.NewReader(data.Encode()))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "PVEAPIToken="+auth.ApiToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return errors.New(fmt.Sprintf("Proxmox API error: %s\nBody: %s", resp.Status, string(body)))
	}

	fmt.Printf("Success: %s\n", body)
	return nil
}
