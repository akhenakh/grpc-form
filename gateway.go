package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"slices"
	"strings"

	"buf.build/go/protovalidate"
	"github.com/jhump/protoreflect/grpcreflect"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/dynamicpb"
)

// NewGateway creates and connects the reflection clients.
func NewGateway(ctx context.Context, services []ServiceConfig, logger *slog.Logger, devMode bool) (*Gateway, error) {
	gw := &Gateway{
		services: services,
		clients:  make(map[string]*grpcreflect.Client),
		channels: make(map[string]*grpc.ClientConn),
		logger:   logger,
		devMode:  devMode,
	}

	for _, svc := range services {
		conn, err := grpc.NewClient(svc.Target, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			logger.Error("failed to dial target", "service_id", svc.ID, "target", svc.Target, "error", err)
			continue
		}
		gw.channels[svc.ID] = conn
		gw.clients[svc.ID] = grpcreflect.NewClientAuto(ctx, conn)
		logger.Info("connected to backend", "service_id", svc.ID, "target", svc.Target)
	}

	return gw, nil
}

// Close cleans up gRPC connections.
func (g *Gateway) Close() {
	for id, conn := range g.channels {
		if err := conn.Close(); err != nil {
			g.logger.Error("failed to close grpc connection", "service_id", id, "error", err)
		}
	}
}

// securityHeadersMiddleware adds HTTP security headers to all responses.
func securityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self'; connect-src 'self'; frame-ancestors 'none'; base-uri 'self'; form-action 'self'")
		next.ServeHTTP(w, r)
	})
}

// router sets up the HTTP multiplexer.
func (g *Gateway) router() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if _, err := w.Write(indexHTML); err != nil {
			g.logger.Error("failed to write response", "error", err)
		}
	})
	mux.HandleFunc("/assets/validate.css", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
		if _, err := w.Write(validateCSS); err != nil {
			g.logger.Error("failed to write response", "error", err)
		}
	})
	mux.HandleFunc("/api/methods", g.handleListMethods)
	mux.HandleFunc("/api/schema", g.handleGetSchema)
	mux.HandleFunc("/api/invoke", g.handleInvoke)
	return securityHeadersMiddleware(mux)
}

func parseMethodName(methodName string) (svcName, mName string, ok bool) {
	idx := strings.LastIndex(methodName, "/")
	if idx <= 0 || idx >= len(methodName)-1 {
		return "", "", false
	}
	return methodName[:idx], methodName[idx+1:], true
}

func (g *Gateway) handleListMethods(w http.ResponseWriter, r *http.Request) {
	userRole := r.Header.Get("X-User-Role")
	if userRole == "" {
		if g.devMode {
			userRole = "admin"
		} else {
			http.Error(w, "Unauthorized: missing X-User-Role header", http.StatusUnauthorized)
			return
		}
	}

	var allowedMethods []map[string]any
	for _, svc := range g.services {
		if slices.Contains(svc.Roles, userRole) {
			for _, m := range svc.Methods {
				methodInfo := map[string]any{
					"service_id": svc.ID,
					"method":     m,
				}

				client, ok := g.clients[svc.ID]
				if ok {
					svcName, mName, valid := parseMethodName(m)
					if !valid {
						g.logger.Warn("invalid method format in config", "method", m)
						continue
					}

					if svcDesc, err := client.ResolveService(svcName); err == nil {
						if methodDesc := svcDesc.FindMethodByName(mName); methodDesc != nil {
							inputDesc := methodDesc.GetInputType().UnwrapMessage()
							title, description := getMessageOptions(inputDesc)
							if title != "" {
								methodInfo["title"] = title
							}
							if description != "" {
								methodInfo["description"] = description
							}
						}
					}
				}

				allowedMethods = append(allowedMethods, methodInfo)
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(allowedMethods); err != nil {
		g.logger.Error("failed to encode response", "error", err)
	}
}

func isValidServiceMethod(serviceID, methodName string, services []ServiceConfig) bool {
	for _, svc := range services {
		if svc.ID == serviceID {
			for _, m := range svc.Methods {
				if m == methodName {
					return true
				}
			}
		}
	}
	return false
}

func (g *Gateway) handleGetSchema(w http.ResponseWriter, r *http.Request) {
	userRole := r.Header.Get("X-User-Role")
	if userRole == "" && !g.devMode {
		http.Error(w, "Unauthorized: missing X-User-Role header", http.StatusUnauthorized)
		return
	}

	serviceID := r.URL.Query().Get("service_id")
	methodName := r.URL.Query().Get("method")

	if serviceID == "" || methodName == "" {
		http.Error(w, "service_id and method parameters are required", http.StatusBadRequest)
		return
	}

	client, ok := g.clients[serviceID]
	if !ok {
		http.Error(w, "Service ID not found", http.StatusNotFound)
		return
	}

	if !isValidServiceMethod(serviceID, methodName, g.services) {
		http.Error(w, "Invalid service or method", http.StatusBadRequest)
		return
	}

	svcName, mName, valid := parseMethodName(methodName)
	if !valid {
		http.Error(w, "Invalid method format, expected 'package.Service/Method'", http.StatusBadRequest)
		return
	}

	svcDesc, err := client.ResolveService(svcName)
	if err != nil {
		g.logger.Error("reflection failed to resolve service", "service", svcName, "error", err)
		http.Error(w, "Service not found", http.StatusNotFound)
		return
	}

	methodDesc := svcDesc.FindMethodByName(mName)
	if methodDesc == nil {
		http.Error(w, "Method not found", http.StatusNotFound)
		return
	}

	inputDesc := methodDesc.GetInputType().UnwrapMessage()
	schema := buildUISchema(inputDesc)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(schema); err != nil {
		g.logger.Error("failed to encode response", "error", err)
	}
}

func (g *Gateway) handleInvoke(w http.ResponseWriter, r *http.Request) {
	userRole := r.Header.Get("X-User-Role")
	if userRole == "" {
		if !g.devMode {
			http.Error(w, "Unauthorized: missing X-User-Role header", http.StatusUnauthorized)
			return
		}
		userRole = "admin"
	}

	serviceID := r.URL.Query().Get("service_id")
	methodName := r.URL.Query().Get("method")

	client, ok := g.clients[serviceID]
	if !ok {
		http.Error(w, "Service ID not found", http.StatusNotFound)
		return
	}

	// Check if the user's role has access to this method
	allowed := false
	for _, svc := range g.services {
		if svc.ID == serviceID {
			if slices.Contains(svc.Roles, userRole) {
				if slices.Contains(svc.Methods, methodName) {
					allowed = true
				}
			}
			break
		}
	}
	if !allowed {
		http.Error(w, "Forbidden: role not authorized for this method", http.StatusForbidden)
		return
	}

	svcName, mName, valid := parseMethodName(methodName)
	if !valid {
		http.Error(w, "Invalid method format, expected 'package.Service/Method'", http.StatusBadRequest)
		return
	}

	svcDesc, err := client.ResolveService(svcName)
	if err != nil {
		g.logger.Error("reflection failed to resolve service", "service", svcName, "error", err)
		http.Error(w, "Service not found", http.StatusNotFound)
		return
	}
	methodDesc := svcDesc.FindMethodByName(mName)
	if methodDesc == nil {
		http.Error(w, "Method not found", http.StatusNotFound)
		return
	}
	unwrappedMethod := methodDesc.UnwrapMethod()

	body, _ := io.ReadAll(r.Body)

	// Create dynamic protobuf message
	reqMsg := dynamicpb.NewMessage(unwrappedMethod.Input())
	respMsg := dynamicpb.NewMessage(unwrappedMethod.Output())

	if err := protojson.Unmarshal(body, reqMsg); err != nil {
		g.logger.Warn("invalid JSON payload", "error", err)
		http.Error(w, fmt.Sprintf("Invalid JSON payload: %v", err), http.StatusBadRequest)
		return
	}

	// Server-side validation using Protovalidate
	v, err := protovalidate.New()
	if err != nil {
		g.logger.Error("failed to initialize validator", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	if err := v.Validate(reqMsg); err != nil {
		g.logger.Warn("validation failed", "method", methodName, "role", userRole, "violations", err.Error())
		w.WriteHeader(http.StatusBadRequest)
		w.Header().Set("Content-Type", "application/json")
		if encErr := json.NewEncoder(w).Encode(map[string]any{"violations": err.Error()}); encErr != nil {
			g.logger.Error("failed to encode validation error", "error", encErr)
		}
		return
	}

	// Invoke remote gRPC service directly
	fullMethodName := fmt.Sprintf("/%s/%s", svcName, mName)

	// Audit log: invocation started
	g.logger.Info("audit: invocation started",
		"event", "invoke_start",
		"role", userRole,
		"service_id", serviceID,
		"method", methodName,
		"request", string(body),
	)

	err = g.channels[serviceID].Invoke(r.Context(), fullMethodName, reqMsg, respMsg)
	if err != nil {
		g.logger.Error("audit: invocation failed",
			"event", "invoke_error",
			"role", userRole,
			"service_id", serviceID,
			"method", methodName,
			"error", err.Error(),
		)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Return response to UI
	respJson, _ := protojson.MarshalOptions{EmitUnpopulated: true}.Marshal(respMsg)
	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write(respJson); err != nil {
		g.logger.Error("failed to write response", "error", err)
	}

	// Audit log: invocation completed
	g.logger.Info("audit: invocation completed",
		"event", "invoke_success",
		"role", userRole,
		"service_id", serviceID,
		"method", methodName,
		"response", string(respJson),
	)
}
