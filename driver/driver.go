package driver

import (
	"context"
	"custom-driver/driver/hostmanager"
	"os"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/spf13/afero"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/klog/v2"
	"k8s.io/utils/mount"
)

type nodeServer struct {
	csi.UnimplementedNodeServer
	agentManager hostmanager.AgentHostManager
	appManager   hostmanager.AppHostManager
}

func NewNodeServer() *nodeServer {
	fs := afero.Afero{Fs: afero.NewOsFs()}
	mounter := mount.New("")
	return &nodeServer{
		agentManager: hostmanager.NewAgentHostManager(fs, mounter),
		appManager:   hostmanager.NewAppHostManager(fs, mounter),
	}
}

var _ csi.NodeServer = &nodeServer{}

func (ns *nodeServer) NodeGetCapabilities(ctx context.Context, req *csi.NodeGetCapabilitiesRequest) (*csi.NodeGetCapabilitiesResponse, error) {
	return &csi.NodeGetCapabilitiesResponse{
		Capabilities: []*csi.NodeServiceCapability{},
	}, nil
}

// NodeGetInfo returns information about the node
func (ns *nodeServer) NodeGetInfo(ctx context.Context, req *csi.NodeGetInfoRequest) (*csi.NodeGetInfoResponse, error) {
	return &csi.NodeGetInfoResponse{
		NodeId: os.Getenv("NODE_ID"), // This should be a unique identifier for the node
		// Optional: include MaxVolumesPerNode if your driver wants to report maximum supported volumes
	}, nil
}

// NodePublishVolume mounts the volume at the target path
func (ns *nodeServer) NodePublishVolume(
	ctx context.Context,
	req *csi.NodePublishVolumeRequest,
) (*csi.NodePublishVolumeResponse, error) {

	targetPath := req.GetTargetPath()
	volumeID := req.GetVolumeId()

	if targetPath == "" || volumeID == "" {
		return nil, status.Error(codes.InvalidArgument, "Target path and Volume ID are required")
	}

	volumeCtx := req.GetVolumeContext()

	volumeType, found := volumeCtx["type"]
	if !found {
		return nil, status.Error(codes.InvalidArgument, "Volume context should include volume mode key")
	}

	var err error = nil
	switch volumeType {
	case SocketVolume, LocalVolume:
		path, foundPath := volumeCtx["path"]
		if foundPath {
			klog.Infof("Requesting mounting %q on %q", targetPath, path)
			err = ns.agentManager.Mount(targetPath, path, volumeType == SocketVolume)
		} else {
			err = status.Errorf(codes.InvalidArgument, "Volume type %q should also specify the 'path' parameter", volumeType)
		}
	case APM:
		err = ns.appManager.Mount(volumeID, targetPath)
	default:
		err = status.Errorf(codes.InvalidArgument, "Unsupported volume type %q. Volume type should be: local, socket or apm", volumeType)
	}

	if err != nil {
		return nil, err
	}
	return &csi.NodePublishVolumeResponse{}, nil
}

// NodeUnpublishVolume unmounts the volume from the target path
func (ns *nodeServer) NodeUnpublishVolume(
	ctx context.Context,
	req *csi.NodeUnpublishVolumeRequest,
) (*csi.NodeUnpublishVolumeResponse, error) {

	targetPath := req.GetTargetPath()
	if err := ns.agentManager.Unmount(targetPath); err != nil {
		return nil, err
	}

	return &csi.NodeUnpublishVolumeResponse{}, nil
}
