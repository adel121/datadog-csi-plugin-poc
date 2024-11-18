package main

import (
	"custom-driver/driver"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc"
	"k8s.io/klog/v2"
)

func main() {
	nodeID := os.Getenv("NODE_ID") // Node ID should be unique and retrievable from the environment
	if nodeID == "" {
		klog.Fatal("NODE_ID environment variable is required")
	}

	// Setup a GRPC server
	endpoint := "/var/lib/kubelet/plugins/example.csi/driver/csi.sock"

	listener, err := net.Listen("unix", endpoint)
	if err != nil {
		klog.Fatalf("Failed to listen: %v", err)
	}

	// Create GRPC servers for our services
	grpcServer := grpc.NewServer()
	identity := driver.NewIdentityServer() // Your identity server instance
	node := driver.NewNodeServer()         // Your node server instance; implement functions as earlier

	csi.RegisterIdentityServer(grpcServer, identity)
	csi.RegisterNodeServer(grpcServer, node)

	// Graceful shutdown handling
	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt, syscall.SIGTERM)
		<-c
		grpcServer.GracefulStop()
	}()

	// Start the server
	klog.Info("Starting GRPC server for CSI driver")
	if err := grpcServer.Serve(listener); err != nil {
		klog.Fatalf("Failed to serve: %v", err)
	}
}
