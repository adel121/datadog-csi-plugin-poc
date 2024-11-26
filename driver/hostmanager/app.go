package hostmanager

import (
	"fmt"
	"os"
	"path"

	"github.com/spf13/afero"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/klog"
	"k8s.io/utils/mount"
)

type AppHostManager struct {
	fs      afero.Afero
	mounter mount.Interface
}

func (hm AppHostManager) Mount(volumeID string, targetPath string) error {
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
		if err := makeFile(hm.fs, dir, false); err != nil {
			return fmt.Errorf("failed to create required directory %q: %w", dir, err)
		}
	}

	notMnt, err := hm.mounter.IsLikelyNotMountPoint(targetPath)
	if err != nil && !os.IsNotExist(err) {
		return status.Errorf(codes.Internal, "Error checking mount point: %v", err)
	}

	if notMnt {
		opts := []string{
			"lowerdir=" + datadogDir,
			"upperdir=" + upperDir,
			"workdir=" + workDir,
		}

		if err := hm.mounter.Mount("overlay", mappedDir, "overlay", opts); err != nil {
			klog.Errorf("Failed to mount overlay %q to %q: %v", datadogDir, mappedDir, err)
			return status.Errorf(codes.Internal, "Failed to mount: %v", err)
		}

		if err := hm.mounter.Mount(mappedDir, targetPath, "", []string{"bind"}); err != nil {
			klog.Errorf("Failed to bind mount %q to %q: %v", mappedDir, targetPath, err)
			return status.Errorf(codes.Internal, "Failed to mount: %v", err)
		}

		if err := recursiveChmod(datadogDir, "777"); err != nil {
			klog.Errorf("Failed to set write permissions for directory %s: %v", targetPath, err)
			return status.Errorf(codes.Internal, "Failed to set write permissions: %v", err)
		}
	}

	return nil
}

func (hm AppHostManager) Unmount(targetPath string) error {
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

func NewAppHostManager(fs afero.Afero, mounter mount.Interface) AppHostManager {
	return AppHostManager{
		fs:      fs,
		mounter: mounter,
	}
}
