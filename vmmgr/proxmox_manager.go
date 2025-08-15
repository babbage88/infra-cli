package vmmgr

import (
	"context"
	"log/slog"
	"time"

	"github.com/babbage88/infra-cli/proxmox"
)

type ProxmoxManager struct {
	Client  *proxmox.Client     `json:"client,omitempty"`
	PveNode proxmox.ProxmoxNode `json:"pveNode"`
}

func NewPveManager(apiUrl string, nodeHostname string, token string, secret string, useTLS bool) (*ProxmoxManager, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	pveMgr := ProxmoxManager{}
	client, err := proxmox.NewClientToken(apiUrl, token, secret, useTLS)
	if err != nil {
		slog.Error("Error initialize PVE manager client", slog.String("apiUrl", apiUrl), slog.String("token", token), slog.String("error", err.Error()))
		return nil, err
	}
	pveMgr.Client = client

	pveMgr.PveNode.QemuVMs, err = client.ListVMs(ctx, nodeHostname, true)
	if err != nil {
		slog.Error("Error retrieving vmlist from PVE Node", slog.String("apiUrl", apiUrl), slog.String("token", token), slog.String("error", err.Error()))
		return nil, err
	}

	return &pveMgr, err
}
