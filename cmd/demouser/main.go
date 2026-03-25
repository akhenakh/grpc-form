package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

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
	enableReflection := os.Getenv("ENABLE_GRPC_REFLECTION") == "true"

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer()

	// Register our Service
	userv1.RegisterUserServiceServer(grpcServer, &server{})

	// Enable gRPC Server Reflection only when explicitly enabled
	// WARNING: Disable in production to prevent service discovery by unauthorized clients
	if enableReflection {
		reflection.Register(grpcServer)
		log.Printf("WARNING: gRPC reflection enabled - not recommended for production")
	}

	log.Printf("Demo gRPC User Service listening on port %d (reflection: %v)...", port, enableReflection)
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
