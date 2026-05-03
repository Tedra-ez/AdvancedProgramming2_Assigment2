package main

import (
	"log"
	"os"

	appointmentapp "appointment-service/app"
)

func main() {
	addr := os.Getenv("GRPC_ADDR")
	if addr == "" {
		addr = ":50052"
	}

	doctorAddr := os.Getenv("DOCTOR_SERVICE_ADDR")
	if doctorAddr == "" {
		doctorAddr = "localhost:50051"
	}

	server, lis, closeDoctorClient, err := appointmentapp.NewServer(addr, doctorAddr)
	if err != nil {
		log.Fatalf("appointment-service setup failed: %v", err)
	}
	defer closeDoctorClient()

	log.Printf("appointment-service gRPC listening on %s", addr)
	if err := server.Serve(lis); err != nil {
		log.Fatalf("appointment-service failed: %v", err)
	}
}
