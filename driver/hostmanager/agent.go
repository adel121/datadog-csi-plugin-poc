package hostmanager

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/afero"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/klog"
	"k8s.io/utils/mount"
)

type AgentHostManager struct {
	fs      afero.Afero
	mounter mount.Interface
}

func (hm AgentHostManager) Mount(targetPath string, hostPath string, isSocketVolume bool) error {
	klog.Infof("Received mount request between %q and %q", targetPath, hostPath)

	if isSocketVolume {
		if !isSocketPath(hostPath) {
			return fmt.Errorf("%q is not a socket file", hostPath)
		}

		hostPath, _ = filepath.Split(hostPath)
	}

	// Check if the target path exists. Create if not present.
	if err := makeFile(hm.fs, targetPath, false); err != nil {
		return fmt.Errorf("failed to create required directory %q: %w", targetPath, err)
	}

	if err := makeFile(hm.fs, hostPath, false); err != nil {
		return fmt.Errorf("failed to create required directory %q: %w", targetPath, err)
	}

	notMnt, err := hm.mounter.IsLikelyNotMountPoint(targetPath)
	if err != nil && !os.IsNotExist(err) {
		return status.Errorf(codes.Internal, "Error checking mount point: %v", err)
	}

	if notMnt {
		klog.Info("tring to mount")
		if err := hm.mounter.Mount(hostPath, targetPath, "", []string{"bind"}); err != nil {
			klog.Errorf("Failed to mount %q to %q: %v", hostPath, targetPath, err)
			return status.Errorf(codes.Internal, "Failed to mount: %v", err)
		}
	}

	return nil
}

func (hm AgentHostManager) Unmount(targetPath string) error {
	// Check if the target path is a mount point. If it's not a mount point, nothing needs to be done.
	isNotMnt, err := hm.mounter.IsLikelyNotMountPoint(targetPath)
	if err != nil {
		if os.IsNotExist(err) {
			// If the target path doesn't exist, there's nothing to unmount,
			// but we return success because from a CSI perspective, the volume is no longer published.
			klog.Info("Target path does not exist, nothing to unmount.")
			return nil
		}
		return status.Errorf(codes.Internal, "Failed to check if target path is a mount point: %v", err)
	}

	// If it's a mount point, proceed to unmount
	if isNotMnt {
		klog.Infof("Target path %s is not a mount point, not doing anything", targetPath)
	} else {
		// Unmount the target path
		if err := hm.mounter.Unmount(targetPath); err != nil {
			return status.Errorf(codes.Internal, "Failed to unmount target path %s: %v", targetPath, err)
		}
	}

	// After unmounting, you may also want to remove the directory to clean up, depending on your use case.
	if err := os.RemoveAll(targetPath); err != nil {
		return status.Errorf(codes.Internal, "Failed to remove target path %s: %v", targetPath, err)
	}

	return nil
}

func NewAgentHostManager(fs afero.Afero, mounter mount.Interface) AgentHostManager {
	return AgentHostManager{
		fs:      fs,
		mounter: mounter,
	}
}
