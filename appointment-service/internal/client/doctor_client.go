package client

import (
	"context"
	"time"

	doctorpb "doctor-service/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

type DoctorClient interface {
	Exists(ctx context.Context, doctorID string) (bool, error)
}

type GRPCDoctorClient struct {
	conn    *grpc.ClientConn
	client  doctorpb.DoctorServiceClient
	timeout time.Duration
}

func NewGRPCDoctorClient(target string, timeout time.Duration) (*GRPCDoctorClient, error) {
	conn, err := grpc.NewClient(target, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}

	return &GRPCDoctorClient{
		conn:    conn,
		client:  doctorpb.NewDoctorServiceClient(conn),
		timeout: timeout,
	}, nil
}

func (c *GRPCDoctorClient) Exists(ctx context.Context, doctorID string) (bool, error) {
	callCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	_, err := c.client.GetDoctor(callCtx, &doctorpb.GetDoctorRequest{Id: doctorID})
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (c *GRPCDoctorClient) Close() error {
	return c.conn.Close()
}
