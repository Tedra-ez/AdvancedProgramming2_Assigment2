package app

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"time"

	"doctor-service/internal/cache"
	"doctor-service/internal/db"
	"doctor-service/internal/event"
	"doctor-service/internal/middleware"
	"doctor-service/internal/repository"
	doctorgrpc "doctor-service/internal/transport/grpc"
	"doctor-service/internal/usecase"
	"doctor-service/internal/usecase/port"
	doctorpb "doctor-service/proto"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

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

	cacheRepo := newDoctorCache()
	limiter, limiterInterceptor := newRateLimiter("doctor-service")

	repo := repository.NewPostgresDoctorRepository(sqlDB)
	uc := usecase.NewDoctorUseCase(repo, publisher, cacheRepo)
	handler := doctorgrpc.NewHandler(uc)

	server := grpc.NewServer(grpc.UnaryInterceptor(limiterInterceptor))
	doctorpb.RegisterDoctorServiceServer(server, handler)
	reflection.Register(server)

	closeFn := func() error {
		_ = publisher.Close()
		_ = cacheRepo.Close()
		if limiter != nil {
			_ = limiter.Close()
		}
		return sqlDB.Close()
	}
	return server, lis, closeFn, nil
}

func newDoctorCache() port.DoctorCacheRepository {
	client := newRedisClient("cache")
	if client == nil {
		return port.NoopDoctorCacheRepository{}
	}
	return cache.NewRedisDoctorCache(client, cacheTTL())
}

func newRateLimiter(prefix string) (*middleware.RedisRateLimiter, grpc.UnaryServerInterceptor) {
	client := newRedisClient("rate_limiter")
	if client == nil {
		return nil, middleware.NoopUnaryInterceptor()
	}
	limiter := middleware.NewRedisRateLimiter(client, rateLimitRPM(), prefix)
	return limiter, limiter.UnaryInterceptor()
}

func newRedisClient(component string) *redis.Client {
	url := os.Getenv("REDIS_URL")
	if url == "" {
		log.Printf("warning: REDIS_URL not set (%s disabled)", component)
		return nil
	}
	opts, err := redis.ParseURL(url)
	if err != nil {
		log.Printf("warning: invalid REDIS_URL (%s disabled): %v", component, err)
		return nil
	}
	client := redis.NewClient(opts)
	ctx, cancel := contextWithTimeout()
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		log.Printf("warning: Redis unavailable (%s disabled): %v", component, err)
		return nil
	}
	return client
}

func cacheTTL() time.Duration {
	seconds := parseEnvInt("CACHE_TTL_SECONDS", 60)
	if seconds <= 0 {
		seconds = 60
	}
	return time.Duration(seconds) * time.Second
}

func rateLimitRPM() int {
	return parseEnvInt("RATE_LIMIT_RPM", 100)
}

func parseEnvInt(key string, fallback int) int {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		log.Printf("warning: invalid %s=%q, using %d", key, raw, fallback)
		return fallback
	}
	return n
}

func contextWithTimeout() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 2*time.Second)
}
