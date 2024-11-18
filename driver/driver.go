package driver

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/spf13/afero"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/klog"
	"k8s.io/utils/mount"
)

type nodeServer struct {
	csi.UnimplementedNodeServer
	mounter mount.Interface
	fs      afero.Afero
}

func NewNodeServer() *nodeServer {
	return &nodeServer{
		mounter: mount.New(""),
		fs:      afero.Afero{Fs: afero.NewOsFs()},
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

	// Paths for OverlayFS
	datadogDir := "/tmp/datadog"

	// Include volumeID in paths to ensure they are unique per volume
	volumeUniqueDir := fmt.Sprintf("/var/lib/csi/overlay/%s", volumeID)
	upperDir := path.Join(volumeUniqueDir, "upper")
	workDir := path.Join(volumeUniqueDir, "work")
	mappedDir := path.Join(volumeUniqueDir, "mapped")

	// Ensure all directories exist
	dirs := []string{datadogDir, upperDir, workDir, targetPath, mappedDir}
	for _, dir := range dirs {
		if err := makeFile(ns.fs, dir); err != nil {
			return nil, fmt.Errorf("failed to create required directory %q: %w", dir, err)
		}
	}

	notMnt, err := ns.mounter.IsLikelyNotMountPoint(targetPath)
	if err != nil && !os.IsNotExist(err) {
		return nil, status.Errorf(codes.Internal, "Error checking mount point: %v", err)
	}

	if notMnt {
		opts := []string{
			"lowerdir=" + datadogDir,
			"upperdir=" + upperDir,
			"workdir=" + workDir,
		}

		if err := ns.mounter.Mount("overlay", mappedDir, "overlay", opts); err != nil {
			klog.Errorf("Failed to mount overlay %q to %q: %v", datadogDir, mappedDir, err)
			return nil, status.Errorf(codes.Internal, "Failed to mount: %v", err)
		}

		if err := ns.mounter.Mount(mappedDir, targetPath, "", []string{"bind"}); err != nil {
			klog.Errorf("Failed to bind mount %q to %q: %v", mappedDir, targetPath, err)
			return nil, status.Errorf(codes.Internal, "Failed to mount: %v", err)
		}

		if err := RecursiveChmod(datadogDir, "777"); err != nil {
			klog.Errorf("Failed to set write permissions for directory %s: %v", targetPath, err)
			return nil, status.Errorf(codes.Internal, "Failed to set write permissions: %v", err)
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
func makeFile(fs afero.Afero, pathname string) error {
	klog.Infof("Checking if directory %s exists", pathname)
	if _, err := os.Stat(pathname); os.IsNotExist(err) {
		klog.Infof("Directory %s does not exist, creating...", pathname)
		if err := fs.MkdirAll(pathname, os.ModePerm); err != nil {
			klog.Errorf("Failed to create directory %s: %v", pathname, err)
			return status.Errorf(codes.Internal, "Cannot create directory: %v", err)
		}
		// Setting permissions again in case umask interfered
		if err := fs.Chmod(pathname, os.ModePerm); err != nil {
			klog.Errorf("Failed to set permissions for directory %s: %v", pathname, err)
			return status.Errorf(codes.Internal, "Cannot set permissions: %v", err)
		}
		klog.Infof("Successfully created and set permissions for directory %s", pathname)
	} else if err != nil {
		klog.Errorf("Error checking directory %s: %v", pathname, err)
		return status.Errorf(codes.Internal, "Error checking directory: %v", err)
	} else {
		klog.Infof("Directory %s already exists", pathname)
	}
	return nil
}

// RecursiveChmod uses the system 'chmod' command to change permissions recursively.
func RecursiveChmod(path string, mode string) error {
	cmd := exec.Command("chmod", "-R", mode, path)
	err := cmd.Run() // Runs the command and waits for it to complete
	if err != nil {
		return err
	}
	return nil
}
