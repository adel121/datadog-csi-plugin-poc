package driver

import (
	"context"
	"fmt"
	"os"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/klog"
	"k8s.io/utils/mount"
)

type nodeServer struct {
	csi.UnimplementedNodeServer
	mounter mount.Interface
}

func NewNodeServer() *nodeServer {
	return &nodeServer{
		mounter: mount.New(""),
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

	klog.Infof("target Path = %q", targetPath)

	// Check if the target path exists. Create if not present.
	_, err := os.Lstat(targetPath)
	if os.IsNotExist(err) {
		if err = makeFile(targetPath); err != nil {
			return nil, fmt.Errorf("failed to create target path: %w", err)
		}
	}
	if err != nil {
		return nil, fmt.Errorf("failed to check if the target block file exists: %w", err)
	}

	datadogDir := "/tmp/datadog"

	klog.Infof("Checking if directory %s exists", datadogDir)
	if _, err := os.Stat(datadogDir); os.IsNotExist(err) {
		klog.Infof("Directory %s does not exist, creating...", datadogDir)
		if err := os.MkdirAll(datadogDir, 0755); err != nil {
			klog.Errorf("Failed to create directory %s: %v", datadogDir, err)
			return nil, status.Errorf(codes.Internal, "Cannot create directory: %v", err)
		}
		klog.Infof("Successfully created directory %s", datadogDir)
	} else if err != nil {
		klog.Errorf("Error checking directory %s: %v", datadogDir, err)
		return nil, status.Errorf(codes.Internal, "Error checking directory: %v", err)
	} else {
		klog.Infof("Directory %s already exists", datadogDir)
	}

	notMnt, err := ns.mounter.IsLikelyNotMountPoint(targetPath)
	if err != nil && !os.IsNotExist(err) {
		return nil, status.Errorf(codes.Internal, "Error checking mount point: %v", err)
	}

	if notMnt {
		if err := ns.mounter.Mount(datadogDir, targetPath, "", []string{"bind"}); err != nil {
			klog.Errorf("Failed to mount %q to %q: %v", datadogDir, targetPath, err)
			return nil, status.Errorf(codes.Internal, "Failed to mount: %v", err)
		}
	}

	return &csi.NodePublishVolumeResponse{}, nil
}

// NodeUnpublishVolume unmounts the volume from the target path
func (ns *nodeServer) NodeUnpublishVolume(
	ctx context.Context,
	req *csi.NodeUnpublishVolumeRequest,
) (*csi.NodeUnpublishVolumeResponse, error) {

	targetPath := req.GetTargetPath()
	if targetPath == "" {
		return nil, status.Error(codes.InvalidArgument, "Target path required")
	}

	// Check if the target path is a mount point. If it's not a mount point, nothing needs to be done.
	isNotMnt, err := ns.mounter.IsLikelyNotMountPoint(targetPath)
	if err != nil {
		if os.IsNotExist(err) {
			// If the target path doesn't exist, there's nothing to unmount,
			// but we return success because from a CSI perspective, the volume is no longer published.
			klog.Info("Target path does not exist, nothing to unmount.")
			return &csi.NodeUnpublishVolumeResponse{}, nil
		}
		return nil, status.Errorf(codes.Internal, "Failed to check if target path is a mount point: %v", err)
	}

	// If it's a mount point, proceed to unmount
	if isNotMnt {
		klog.Infof("Target path %s is not a mount point, not doing anything", targetPath)
	} else {
		// Unmount the target path
		if err := ns.mounter.Unmount(targetPath); err != nil {
			return nil, status.Errorf(codes.Internal, "Failed to unmount target path %s: %v", targetPath, err)
		}
	}

	// After unmounting, you may also want to remove the directory to clean up, depending on your use case.
	if err := os.RemoveAll(targetPath); err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to remove target path %s: %v", targetPath, err)
	}

	return &csi.NodeUnpublishVolumeResponse{}, nil
}

// makeFile ensures that the file exists, creating it if necessary.
// The parent directory must exist.
func makeFile(pathname string) error {
	klog.Infof("Checking if directory %s exists", pathname)
	if _, err := os.Stat(pathname); os.IsNotExist(err) {
		klog.Infof("Directory %s does not exist, creating...", pathname)
		if err := os.MkdirAll(pathname, 0755); err != nil {
			klog.Errorf("Failed to create directory %s: %v", pathname, err)
			return status.Errorf(codes.Internal, "Cannot create directory: %v", err)
		}
		klog.Infof("Successfully created directory %s", pathname)
	} else if err != nil {
		klog.Errorf("Error checking directory %s: %v", pathname, err)
		return status.Errorf(codes.Internal, "Error checking directory: %v", err)
	} else {
		klog.Infof("Directory %s already exists", pathname)
	}

	return nil
}
