package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"buf.build/go/protovalidate"
	"github.com/caarlos0/env/v11"
	"github.com/jhump/protoreflect/grpcreflect"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
	"gopkg.in/yaml.v3"

	formv1 "github.com/akhenakh/grpc-form/gen/form/v1"
)

const appName = "admin-form-gateway"

//go:embed index.html
var indexHTML []byte

//go:embed validate.css
var validateCSS []byte

// EnvConfig holds server-level configuration from environment variables.
type EnvConfig struct {
	LogLevel     string `env:"LOG_LEVEL" envDefault:"INFO"`
	HTTPPort     int    `env:"HTTP_PORT" envDefault:"8080"`
	ConfigPath   string `env:"CONFIG_PATH" envDefault:"config.yaml"`
	AllowDevMode bool   `env:"ALLOW_DEV_MODE" envDefault:"false"`
}

// YAMLConfig holds the dynamic routing and role rules for the gateway.
type YAMLConfig struct {
	Services []ServiceConfig `yaml:"services"`
}

type ServiceConfig struct {
	ID      string   `yaml:"id"`
	Name    string   `yaml:"name"`
	Target  string   `yaml:"target"`
	Methods []string `yaml:"methods"`
	Roles   []string `yaml:"roles"`
}

// Gateway encapsulates the state of our API Gateway.
type Gateway struct {
	services []ServiceConfig
	clients  map[string]*grpcreflect.Client
	channels map[string]*grpc.ClientConn
	logger   *slog.Logger
	devMode  bool
}

func main() {
	var envCfg EnvConfig
	if err := env.Parse(&envCfg); err != nil {
		fmt.Printf("failed to parse env config: %+v\n", err)
		os.Exit(1)
	}

	logger := createLogger(envCfg)
	slog.SetDefault(logger)

	// 1. Read YAML routes config
	yamlData, err := os.ReadFile(envCfg.ConfigPath)
	if err != nil {
		logger.Error("failed to read yaml config", "path", envCfg.ConfigPath, "error", err)
		os.Exit(1)
	}
	var yamlCfg YAMLConfig
	if err := yaml.Unmarshal(yamlData, &yamlCfg); err != nil {
		logger.Error("failed to parse yaml config", "error", err)
		os.Exit(1)
	}

	// 2. Setup Context and Signals
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(interrupt)

	g, gCtx := errgroup.WithContext(ctx)

	// 3. Initialize Gateway state & gRPC clients
	gw, err := NewGateway(ctx, yamlCfg.Services, logger, envCfg.AllowDevMode)
	if err != nil {
		logger.Error("failed to initialize gateway", "error", err)
		os.Exit(1)
	}
	defer gw.Close()

	// 4. Start HTTP Server
	httpServer := &http.Server{
		Addr:    fmt.Sprintf(":%d", envCfg.HTTPPort),
		Handler: gw.router(),
	}

	g.Go(func() error {
		logger.Info("Admin Form Gateway listening", "port", envCfg.HTTPPort)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("HTTP server failed: %w", err)
		}
		return nil
	})

	// 5. Wait for termination signal or an error from the server
	select {
	case <-interrupt:
		logger.Warn("received termination signal, starting graceful shutdown")
		cancel()
	case <-gCtx.Done():
		logger.Warn("context cancelled, starting graceful shutdown")
	}

	// 6. Graceful Shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		logger.Error("HTTP server shutdown error", "error", err)
	}

	// Wait for all services in the errgroup to finish
	if err := g.Wait(); err != nil && !errors.Is(err, context.Canceled) {
		logger.Error("server group returned an error", "error", err)
		os.Exit(2)
	}
}

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
		conn, err := grpc.DialContext(ctx, svc.Target, grpc.WithTransportCredentials(insecure.NewCredentials()))
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

// router sets up the HTTP multiplexer.
func (g *Gateway) router() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(indexHTML)
	})
	mux.HandleFunc("/assets/validate.css", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
		w.Write(validateCSS)
	})
	mux.HandleFunc("/api/methods", g.handleListMethods)
	mux.HandleFunc("/api/schema", g.handleGetSchema)
	mux.HandleFunc("/api/invoke", g.handleInvoke)
	return mux
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

	var allowedMethods []map[string]string
	for _, svc := range g.services {
		if contains(svc.Roles, userRole) {
			for _, m := range svc.Methods {
				allowedMethods = append(allowedMethods, map[string]string{
					"service_id": svc.ID,
					"method":     m,
				})
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(allowedMethods)
}

func (g *Gateway) handleGetSchema(w http.ResponseWriter, r *http.Request) {
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

	svcName := methodName[:strings.LastIndex(methodName, "/")]
	mName := methodName[strings.LastIndex(methodName, "/")+1:]

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
	json.NewEncoder(w).Encode(schema)
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
			if contains(svc.Roles, userRole) {
				for _, m := range svc.Methods {
					if m == methodName {
						allowed = true
						break
					}
				}
			}
			break
		}
	}
	if !allowed {
		http.Error(w, "Forbidden: role not authorized for this method", http.StatusForbidden)
		return
	}

	svcName := methodName[:strings.LastIndex(methodName, "/")]
	mName := methodName[strings.LastIndex(methodName, "/")+1:]

	svcDesc, _ := client.ResolveService(svcName)
	methodDesc := svcDesc.FindMethodByName(mName).UnwrapMethod()

	body, _ := io.ReadAll(r.Body)

	// 1. Create dynamic protobuf message
	reqMsg := dynamicpb.NewMessage(methodDesc.Input())
	respMsg := dynamicpb.NewMessage(methodDesc.Output())

	if err := protojson.Unmarshal(body, reqMsg); err != nil {
		g.logger.Warn("invalid JSON payload", "error", err)
		http.Error(w, fmt.Sprintf("Invalid JSON payload: %v", err), http.StatusBadRequest)
		return
	}

	// 2. Server-side validation using Protovalidate
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
		json.NewEncoder(w).Encode(map[string]interface{}{"violations": err.Error()})
		return
	}

	// 3. Invoke remote gRPC service directly
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

	// 4. Return response to UI
	respJson, _ := protojson.MarshalOptions{EmitUnpopulated: true}.Marshal(respMsg)
	w.Header().Set("Content-Type", "application/json")
	w.Write(respJson)

	// Audit log: invocation completed
	g.logger.Info("audit: invocation completed",
		"event", "invoke_success",
		"role", userRole,
		"service_id", serviceID,
		"method", methodName,
		"response", string(respJson),
	)
}

// --- Schema Builders ---

func buildUISchema(msg protoreflect.MessageDescriptor) map[string]interface{} {
	title, desc := getMessageOptions(msg)
	schema := map[string]interface{}{
		"title":       title,
		"description": desc,
		"fields":      []map[string]interface{}{},
		"enums":       map[string]interface{}{},
		"messages":    map[string]interface{}{},
	}

	fields := msg.Fields()
	for i := 0; i < fields.Len(); i++ {
		field := fields.Get(i)
		schema["fields"] = append(schema["fields"].([]map[string]interface{}), buildFieldSchema(schema, field))
	}

	return schema
}

func getMessageOptions(msg protoreflect.MessageDescriptor) (title, description string) {
	opts := msg.Options().(*descriptorpb.MessageOptions)
	if opts == nil {
		return
	}
	title, _ = proto.GetExtension(opts, formv1.E_Title).(string)
	description, _ = proto.GetExtension(opts, formv1.E_Description).(string)
	return
}

func buildFieldSchema(rootSchema map[string]interface{}, field protoreflect.FieldDescriptor) map[string]interface{} {
	fType := field.Kind().String()

	if field.Kind() == protoreflect.MessageKind {
		fType = string(field.Message().FullName())
		populateMessages(rootSchema, field.Message())
	} else if field.Kind() == protoreflect.EnumKind {
		fType = string(field.Enum().FullName())
		populateEnums(rootSchema, field.Enum())
	}

	label := strings.Title(strings.ReplaceAll(string(field.Name()), "_", " "))
	hidden := false
	placeholder := ""
	hint := ""

	if opts := field.Options().(*descriptorpb.FieldOptions); opts != nil {
		if ext := proto.GetExtension(opts, formv1.E_Field); ext != nil {
			if fo, ok := ext.(*formv1.FieldOptions); ok {
				if fo.Label != "" {
					label = fo.Label
				}
				hidden = fo.Hidden
				placeholder = fo.Placeholder
				hint = fo.Hint
			}
		}
	}

	fSchema := map[string]interface{}{
		"name":        string(field.Name()),
		"type":        fType,
		"repeated":    field.IsList(),
		"label":       label,
		"hidden":      hidden,
		"placeholder": placeholder,
		"hint":        hint,
		"validate":    map[string]interface{}{},
		"isEnum":      field.Kind() == protoreflect.EnumKind,
		"isMessage":   field.Kind() == protoreflect.MessageKind,
	}

	if field.ContainingOneof() != nil {
		fSchema["oneofGroup"] = string(field.ContainingOneof().Name())
	}

	return fSchema
}

func populateMessages(rootSchema map[string]interface{}, msg protoreflect.MessageDescriptor) {
	msgName := string(msg.FullName())
	messagesMap := rootSchema["messages"].(map[string]interface{})

	if _, exists := messagesMap[msgName]; exists {
		return
	}
	messagesMap[msgName] = []map[string]interface{}{}

	subFields := []map[string]interface{}{}
	fields := msg.Fields()
	for i := 0; i < fields.Len(); i++ {
		subFields = append(subFields, buildFieldSchema(rootSchema, fields.Get(i)))
	}
	messagesMap[msgName] = subFields
}

func populateEnums(rootSchema map[string]interface{}, enum protoreflect.EnumDescriptor) {
	enumName := string(enum.FullName())
	enumsMap := rootSchema["enums"].(map[string]interface{})

	if _, exists := enumsMap[enumName]; exists {
		return
	}

	vals := []map[string]interface{}{}
	enumValues := enum.Values()
	for i := 0; i < enumValues.Len(); i++ {
		v := enumValues.Get(i)
		vals = append(vals, map[string]interface{}{
			"name":   string(v.Name()),
			"number": int32(v.Number()),
		})
	}
	enumsMap[enumName] = vals
}

// --- Utilities ---

func createLogger(cfg EnvConfig) *slog.Logger {
	level := slog.LevelInfo
	if strings.ToUpper(cfg.LogLevel) == "DEBUG" {
		level = slog.LevelDebug
	}

	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	})
	return slog.New(handler).With("app", appName)
}

func contains(slice []string, val string) bool {
	for _, item := range slice {
		if item == val {
			return true
		}
	}
	return false
}
