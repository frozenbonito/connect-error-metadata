package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"

	"connectrpc.com/connect"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/frozenbonito/connect-error-metadata/pb"
	"github.com/frozenbonito/connect-error-metadata/pb/pbconnect"
)

const metadataKey = "custom-key"

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	fmt.Println("Connect")
	if err := runConnect(); err != nil {
		return fmt.Errorf("run connect: %w", err)
	}

	fmt.Println("-----------------")

	fmt.Println("gRPC")
	if err := runGRPC(); err != nil {
		return fmt.Errorf("run grpc: %w", err)
	}

	return nil
}

type connectServer struct {
	pbconnect.UnimplementedGreeterHandler
}

func (*connectServer) SayHello(_ context.Context, _ *connect.Request[pb.HelloRequest]) (*connect.Response[pb.HelloReply], error) {
	err := connect.NewError(connect.CodeUnimplemented, errors.New("unimplemented"))

	err.Meta().Set(metadataKey, "value")

	return nil, err
}

func runConnect() error {
	mux := http.NewServeMux()
	mux.Handle(pbconnect.NewGreeterHandler(&connectServer{}))
	srv := &http.Server{
		Handler: h2c.NewHandler(mux, &http2.Server{}),
	}

	lis, err := net.Listen("tcp", ":8080")
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}

	go srv.Serve(lis)
	defer srv.Close()

	if err := request(); err != nil {
		return fmt.Errorf("request: %w", err)
	}

	return nil
}

type grpcServer struct {
	pb.UnimplementedGreeterServer
}

func (*grpcServer) SayHello(ctx context.Context, _ *pb.HelloRequest) (*pb.HelloReply, error) {
	md := metadata.New(nil)
	md.Set(metadataKey, "value")
	grpc.SetHeader(ctx, md)
	return nil, status.Error(codes.Unimplemented, "unimplemented")
}

func runGRPC() error {
	srv := grpc.NewServer()
	pb.RegisterGreeterServer(srv, &grpcServer{})

	lis, err := net.Listen("tcp", ":8080")
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}

	go srv.Serve(lis)
	defer srv.Stop()

	if err := request(); err != nil {
		return fmt.Errorf("request: %w", err)
	}

	return nil
}

func request() error {
	conn, err := grpc.NewClient("localhost:8080", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("new client: %w", err)
	}

	client := pb.NewGreeterClient(conn)

	ctx := context.Background()
	req := &pb.HelloRequest{}
	var header, trailer metadata.MD

	_, err = client.SayHello(ctx, req, grpc.Header(&header), grpc.Trailer(&trailer)) // unimplemented error

	fmt.Println(err)
	fmt.Printf("header: %v\n", header)
	fmt.Printf("trailer: %v\n", trailer)

	return nil
}
