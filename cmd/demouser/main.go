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

type server struct {
	userv1.UnimplementedUserServiceServer
}

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

func (s *server) ListUsers(ctx context.Context, req *userv1.ListUsersRequest) (*userv1.ListUsersResponse, error) {
	log.Printf("Received ListUsers request: page_size=%d, page_token=%s", req.GetPageSize(), req.GetPageToken())

	stubUsers := []*userv1.User{
		{
			Id:        "usr-001",
			FirstName: "Alice",
			LastName:  "Johnson",
			Email:     "alice.johnson@example.com",
			Role:      userv1.Role_ROLE_ADMIN,
			Tags:      []string{"engineering", "lead"},
			Age:       32,
			CreatedAt: "2024-01-15T10:30:00Z",
			Status:    "active",
		},
		{
			Id:        "usr-002",
			FirstName: "Bob",
			LastName:  "Smith",
			Email:     "bob.smith@example.com",
			Role:      userv1.Role_ROLE_EDITOR,
			Tags:      []string{"content", "writer"},
			Age:       28,
			CreatedAt: "2024-02-20T14:45:00Z",
			Status:    "active",
		},
		{
			Id:        "usr-003",
			FirstName: "Carol",
			LastName:  "Williams",
			Email:     "carol.williams@example.com",
			Role:      userv1.Role_ROLE_VIEWER,
			Tags:      []string{"sales"},
			Age:       45,
			CreatedAt: "2024-03-10T09:15:00Z",
			Status:    "inactive",
		},
		{
			Id:        "usr-004",
			FirstName: "David",
			LastName:  "Brown",
			Email:     "david.brown@example.com",
			Role:      userv1.Role_ROLE_EDITOR,
			Tags:      []string{"marketing", "seo"},
			Age:       35,
			CreatedAt: "2024-03-25T16:20:00Z",
			Status:    "active",
		},
		{
			Id:        "usr-005",
			FirstName: "Eve",
			LastName:  "Davis",
			Email:     "eve.davis@example.com",
			Role:      userv1.Role_ROLE_VIEWER,
			Tags:      []string{"support"},
			Age:       29,
			CreatedAt: "2024-04-01T11:00:00Z",
			Status:    "pending",
		},
		{
			Id:        "usr-006",
			FirstName: "Frank",
			LastName:  "Miller",
			Email:     "frank.miller@example.com",
			Role:      userv1.Role_ROLE_ADMIN,
			Tags:      []string{"engineering", "backend"},
			Age:       41,
			CreatedAt: "2024-04-15T08:30:00Z",
			Status:    "active",
		},
		{
			Id:        "usr-007",
			FirstName: "Grace",
			LastName:  "Wilson",
			Email:     "grace.wilson@example.com",
			Role:      userv1.Role_ROLE_EDITOR,
			Tags:      []string{"design", "ui/ux"},
			Age:       26,
			CreatedAt: "2024-05-02T13:45:00Z",
			Status:    "active",
		},
		{
			Id:        "usr-008",
			FirstName: "Henry",
			LastName:  "Taylor",
			Email:     "henry.taylor@example.com",
			Role:      userv1.Role_ROLE_VIEWER,
			Tags:      []string{"finance"},
			Age:       52,
			CreatedAt: "2024-05-20T10:10:00Z",
			Status:    "inactive",
		},
	}

	return &userv1.ListUsersResponse{
		Users:         stubUsers,
		NextPageToken: "",
		TotalCount:    int32(len(stubUsers)),
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

	userv1.RegisterUserServiceServer(grpcServer, &server{})

	if enableReflection {
		reflection.Register(grpcServer)
		log.Printf("WARNING: gRPC reflection enabled - not recommended for production")
	}

	log.Printf("Demo gRPC User Service listening on port %d (reflection: %v)...", port, enableReflection)
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
