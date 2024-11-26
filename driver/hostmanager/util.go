package hostmanager

import (
	"io/fs"
	"os"
	"os/exec"

	"github.com/spf13/afero"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/klog"
)

func makeFile(fs afero.Afero, pathname string, isFile bool) error {

	klog.Infof("Checking if path %s exists (isFile=%t)", pathname, isFile)

	// Check if the pathname exists
	if _, err := os.Stat(pathname); os.IsNotExist(err) {
		if pathname == "/var/log/pods" {
			panic("shouldn't be creating /var/log/pods")
		}
		if isFile {
			klog.Infof("File %s does not exist, creating...", pathname)
			// Create the file
			file, err := fs.Create(pathname)
			if err != nil {
				klog.Errorf("Failed to create file %s: %v", pathname, err)
				return status.Errorf(codes.Internal, "Cannot create file: %v", err)
			}
			defer file.Close() // Ensure the file gets closed after creation
			klog.Infof("Successfully created file %s", pathname)
		} else {
			klog.Infof("Directory %s does not exist, creating...", pathname)
			const dirPerm = 0755
			// Create the directory
			if err := fs.MkdirAll(pathname, dirPerm); err != nil {
				klog.Errorf("Failed to create directory %s: %v", pathname, err)
				return status.Errorf(codes.Internal, "Cannot create directory: %v", err)
			}
			// Set permissions explicitly
			if err := fs.Chmod(pathname, dirPerm); err != nil {
				klog.Errorf("Failed to set permissions for directory %s: %v", pathname, err)
				return status.Errorf(codes.Internal, "Cannot set permissions: %v", err)
			}
			klog.Infof("Successfully created and set permissions for directory %s", pathname)
		}
	} else if err != nil {
		klog.Errorf("Error checking path %s: %v", pathname, err)
		return status.Errorf(codes.Internal, "Error checking path: %v", err)
	} else {
		klog.Infof("Path %s already exists", pathname)
	}

	return nil
}

// RecursiveChmod uses the system 'chmod' command to change permissions recursively.
func recursiveChmod(path string, mode string) error {
	cmd := exec.Command("chmod", "-R", mode, path)
	err := cmd.Run() // Runs the command and waits for it to complete
	if err != nil {
		return err
	}
	return nil
}

// Check if a file is a socket.
func isSocketPath(path string) bool {
	fileInfo, err := os.Stat(path)
	if err != nil {
		klog.Fatal("ERROR - ", err)
	}
	return fileInfo.Mode().Type() == fs.ModeSocket
}
