package app

import (
	"database/sql"
	"fmt"
	"log"
	"net"
	"os"

	"doctor-service/internal/db"
	"doctor-service/internal/event"
	doctorgrpc "doctor-service/internal/transport/grpc"
	"doctor-service/internal/usecase/port"
	doctorpb "doctor-service/proto"
	"google.golang.org/grpc"

	"doctor-service/internal/repository"
	"doctor-service/internal/usecase"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func NewServer(addr string) (*grpc.Server, net.Listener, func() error, error) {
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

	var publisher port.DoctorEventPublisher = port.NoopDoctorEventPublisher{}
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

	repo := repository.NewPostgresDoctorRepository(sqlDB)
	uc := usecase.NewDoctorUseCase(repo, publisher)
	handler := doctorgrpc.NewHandler(uc)

	server := grpc.NewServer()
	doctorpb.RegisterDoctorServiceServer(server, handler)

	closeFn := func() error {
		_ = publisher.Close()
		return sqlDB.Close()
	}
	return server, lis, closeFn, nil
}
