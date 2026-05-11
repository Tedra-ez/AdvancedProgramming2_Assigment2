package middleware

import (
	"context"
	"fmt"
	"log"
	"net"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

type RedisRateLimiter struct {
	client *redis.Client
	rpm    int
	prefix string
}

func NewRedisRateLimiter(client *redis.Client, rpm int, prefix string) *RedisRateLimiter {
	if rpm <= 0 {
		rpm = 100
	}
	return &RedisRateLimiter{client: client, rpm: rpm, prefix: prefix}
}

func (l *RedisRateLimiter) UnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		now := time.Now().UTC()
		ip := clientIP(ctx)
		windowStart := now.Truncate(time.Minute)
		key := fmt.Sprintf("rate:%s:%s:%d", l.prefix, ip, windowStart.Unix())

		count, err := l.client.Incr(ctx, key).Result()
		if err != nil {
			log.Printf("rate limit check failed key=%s err=%v", key, err)
			return handler(ctx, req)
		}
		if count == 1 {
			_ = l.client.Expire(ctx, key, time.Minute).Err()
		}
		if count > int64(l.rpm) {
			retryAfter := int(time.Until(windowStart.Add(time.Minute)).Seconds())
			if retryAfter < 1 {
				retryAfter = 1
			}
			return nil, status.Error(codes.ResourceExhausted, "rate limit exceeded; retry after "+strconv.Itoa(retryAfter)+" seconds")
		}
		return handler(ctx, req)
	}
}

func (l *RedisRateLimiter) Close() error {
	return l.client.Close()
}

func clientIP(ctx context.Context) string {
	p, ok := peer.FromContext(ctx)
	if !ok || p.Addr == nil {
		return "unknown"
	}
	host, _, err := net.SplitHostPort(p.Addr.String())
	if err != nil {
		return p.Addr.String()
	}
	return host
}

func NoopUnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		return handler(ctx, req)
	}
}
