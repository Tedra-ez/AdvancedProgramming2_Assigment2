package main

import (
	"log"
	"os"

	doctorapp "doctor-service/app"
)

func main() {
	addr := os.Getenv("GRPC_ADDR")
	if addr == "" {
		addr = ":50051"
	}
	server, lis, closeFn, err := doctorapp.NewServer(addr)
	if err != nil {
		log.Fatalf("doctor-service setup failed: %v", err)
	}
	defer func() { _ = closeFn() }()

	log.Printf("doctor-service gRPC listening on %s", addr)
	if err := server.Serve(lis); err != nil {
		log.Fatalf("doctor-service failed: %v", err)
	}
}
