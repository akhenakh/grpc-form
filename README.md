# gRPC Admin Form Builder

A dynamic admin form builder that automatically generates web forms from gRPC protobuf definitions. The gateway introspects target microservices via gRPC Reflection and builds forms on-the-fly using protovalidate for server-side validation.

## Architecture

This architecture solves your admin form needs with:

- **Dynamic form construction**: grpcreflect ensures you don't need to generate or compile Go stubs whenever you add a new RPC or form. The server fetches the schema layout over the network.

- **Server-side protovalidate**: We inject `protovalidate.New().Validate(dynamicMessage)` in the proxy. Before the request even touches the target microservice, the gateway ensures the form submission accurately matches the .proto validation rules.

- **RBAC Control**: The config.yaml specifies what methods belong to what UI roles. If the JWT presented by your API Gateway maps to `X-User-Role: editor`, the frontend will only populate forms available to the editor role.

- **Custom annotations**: By parsing `form.field.hidden` and UI labels out of the options dynamically, developers writing the target microservices get full control over how their forms appear in the central admin panel without writing any JavaScript.

```sh
protoc --go_out=. --go_opt=paths=source_relative form/v1/annotations.proto
```

## Configuration

The `config.yaml` file controls which gRPC services are exposed and their role-based access:

```yaml
server:
  port: 8080
services:
  - id: "user-service"
    name: "User Management"
    target: "localhost:50051"
    methods:
      - "user.v1.UserService/CreateUser"
    roles: ["admin"]
```

### Fields

| Field | Description |
|-------|-------------|
| `server.port` | HTTP port for the admin gateway |
| `services[].id` | Unique identifier for this service |
| `services[].name` | Display name shown in the UI |
| `services[].target` | gRPC address of the target microservice |
| `services[].methods` | List of RPC methods to expose (fully qualified names) |
| `services[].roles` | Roles that can access these methods |

### Role-Based Access

The gateway reads the `X-User-Role` header to determine the user's role. Only methods assigned to that role will be displayed in the UI.

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `LOG_LEVEL` | INFO | Logging level (DEBUG, INFO, WARN, ERROR) |
| `HTTP_PORT` | 8080 | HTTP server port |
| `CONFIG_PATH` | config.yaml | Path to YAML configuration file |
| `ALLOW_DEV_MODE` | false | When true, allows requests without X-User-Role header (defaults to admin). **Do not enable in production.** |

**Security Note**: By default (`ALLOW_DEV_MODE=false`), the gateway requires a valid `X-User-Role` header. Set `ALLOW_DEV_MODE=true` only for local development.
