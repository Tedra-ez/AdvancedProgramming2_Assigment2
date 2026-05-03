package app

import (
	"database/sql"
	"fmt"
	"log"
	"net"
	"os"
	"time"

	"appointment-service/internal/db"
	"appointment-service/internal/client"
	"appointment-service/internal/event"
	"appointment-service/internal/repository"
	appointmentgrpc "appointment-service/internal/transport/grpc"
	"appointment-service/internal/usecase"
	"appointment-service/internal/usecase/port"
	appointmentpb "appointment-service/proto"
	"google.golang.org/grpc"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func NewServer(addr, doctorServiceAddr string) (*grpc.Server, net.Listener, func() error, error) {
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, nil, nil, err
	}

	dsn := os.Getenv("DB_DSN")
	if dsn == "" {
		dsn = os.Getenv("DATABASE_URL")
	}
	if dsn == "" {
		_ = lis.Close()
		return nil, nil, nil, fmt.Errorf("missing DB_DSN/DATABASE_URL")
	}

	sqlDB, err := sql.Open("pgx", dsn)
	if err != nil {
		_ = lis.Close()
		return nil, nil, nil, fmt.Errorf("open db: %w", err)
	}
	if err := sqlDB.Ping(); err != nil {
		_ = lis.Close()
		_ = sqlDB.Close()
		return nil, nil, nil, fmt.Errorf("ping db: %w", err)
	}
	if err := db.RunMigrations(sqlDB, "./migrations"); err != nil {
		_ = lis.Close()
		_ = sqlDB.Close()
		return nil, nil, nil, err
	}

	var publisher port.AppointmentEventPublisher = port.NoopAppointmentEventPublisher{}
	if natsURL := os.Getenv("NATS_URL"); natsURL != "" {
		p, err := event.NewNATSPublisher(natsURL)
		if err != nil {
			log.Printf("warning: cannot connect to NATS (events best-effort): %v", err)
		} else {
			publisher = p
		}
	} else {
		log.Printf("warning: NATS_URL not set (events disabled)")
	}

	repo := repository.NewPostgresAppointmentRepository(sqlDB)
	doctorClient, err := client.NewGRPCDoctorClient(doctorServiceAddr, 2*time.Second)
	if err != nil {
		_ = lis.Close()
		_ = publisher.Close()
		_ = sqlDB.Close()
		return nil, nil, nil, err
	}
	uc := usecase.NewAppointmentUseCase(repo, doctorClient, publisher)
	handler := appointmentgrpc.NewHandler(uc)

	server := grpc.NewServer()
	appointmentpb.RegisterAppointmentServiceServer(server, handler)

	closeFn := func() error {
		_ = publisher.Close()
		_ = sqlDB.Close()
		return doctorClient.Close()
	}
	return server, lis, closeFn, nil
}
