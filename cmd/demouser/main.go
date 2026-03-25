package main

import (
	"context"
	"fmt"
	"log"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	// Import the generated code
	userv1 "github.com/akhenakh/grpc-form/gen/user/v1"
)

// server implements the UserServiceServer interface
type server struct {
	userv1.UnimplementedUserServiceServer
}

// CreateUser handles the incoming gRPC request
func (s *server) CreateUser(ctx context.Context, req *userv1.CreateUserRequest) (*userv1.CreateUserResponse, error) {
	log.Printf("Received CreateUser request:")
	log.Printf(" - FirstName: %s", req.GetFirstName())
	log.Printf(" - LastName:  %s", req.GetLastName())
	log.Printf(" - Email:     %s", req.GetEmail())
	log.Printf(" - Role:      %v", req.GetRole())
	log.Printf(" - Tags:      %v", req.GetTags())
	log.Printf(" - Age:       %d", req.GetAge())
	log.Printf(" - Address:   %v", req.GetAddress())
	log.Printf(" - Contact:   %v", req.GetContactMethod())

	newID := "user-12345"

	return &userv1.CreateUserResponse{
		Message: fmt.Sprintf("User %s %s successfully created!", req.GetFirstName(), req.GetLastName()),
		UserId:  newID,
	}, nil
}

func main() {
	port := 50051
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer()

	// 1. Register our Service
	userv1.RegisterUserServiceServer(grpcServer, &server{})

	// 2. Enable gRPC Server Reflection (CRITICAL for the API Gateway)
	reflection.Register(grpcServer)

	log.Printf("Demo gRPC User Service listening on port %d...", port)
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
