package driver

import (
	"context"

	"github.com/container-storage-interface/spec/lib/go/csi"
)

// Implement IdentityServer interface from CSI specification
type identityServer struct {
	csi.UnimplementedIdentityServer
}

// GetPluginInfo returns metadata about the plugin
func (is *identityServer) GetPluginInfo(ctx context.Context, req *csi.GetPluginInfoRequest) (*csi.GetPluginInfoResponse, error) {
	return &csi.GetPluginInfoResponse{
		Name:          "datadog.poc.csi.driver",
		VendorVersion: "v1.0.0",
	}, nil
}

func NewIdentityServer() *identityServer {
	return &identityServer{}
}
