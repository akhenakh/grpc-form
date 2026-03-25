# gRPC Admin Form Builder

A user-friendly admin interface that generates web forms from your backend services. No coding required for the admin UI—developers define forms once in their service definitions, and the interface builds itself.

## Who Is This For?

**This tool is designed for:**
- Product managers who need admin panels for their services
- Operations teams managing backend systems
- Support teams who need to perform administrative actions
- Anyone who needs to interact with gRPC services without writing code

**Not for:**
- Developers debugging gRPC services during development (use [grpcui](https://github.com/fullstorydev/grpcui) instead)

## How Is This Different from grpcui?

| Feature | gRPC Admin Form Builder | grpcui |
|---------|------------------------|--------|
| **Target Audience** | Non-technical users (ops, support, product) | Developers debugging services |
| **Form Customization** | Customizable labels, hints, descriptions, ordering | Raw field names from proto |
| **Endpoint Exposure** | Curated list of allowed actions per role | Every service method exposed |
| **User Experience** | Clean, guided forms with help text | Technical JSON/protobuf view |
| **Access Control** | Role-based (admin, editor, viewer) | None—full access to all methods |
| **Hidden Fields** | Can hide fields like internal IDs | Shows all fields |
| **Field Labels** | Human-readable ("First Name") | Technical names ("first_name") |

### When to Use Which

**Use gRPC Admin Form Builder when:**
- You need a production admin panel for non-developers
- You want to restrict what actions different roles can perform
- You want human-readable labels and helpful guidance for users
- You're building aninternal tool for support/ops teams

**Use grpcui when:**
- You're a developer debugging during development
- You need to see every field and every method
- You want to explore a service's full API surface
- You're testing edge cases with raw inputs

## How It Works

### For the User

1. **Select an action** from the curated list of available operations
2. **Fill out the form** with human-readable labels and helpful hints
3. **Submit** — the system validates your input and shows the result

Forms are automatically generated from your developer's service definitions, with friendly labels and validation messages.

### For theDeveloper

Developers enhance their `.proto` files with annotations to control how forms appear:

```protobuf
message CreateUserRequest {
  option (form.v1.title) = "Create User";
  option (form.v1.description) = "Add a new user to the system";

  string first_name = 1 [
    (buf.validate.field).string.min_len = 1,
    (form.v1.field).label = "First name",
    (form.v1.field).placeholder = "Jane"
  ];

  string email = 3 [
    (buf.validate.field).string.email = true,
    (form.v1.field).label = "Email address",
    (form.v1.field).hint = "Must be a valid corporate email"
  ];

  Role role = 4[(form.v1.field).label = "Role"];

  string internal_id = 7[(form.v1.field).hidden = true];
}
```

This produces a form where:
- Title and description guide the user
- Fields have meaningful labels instead of technical names
- Placeholders and hints explain what to enter
- Hidden fields are excluded from the UI
- Validation errors are shown in plain language

## Configuration

Administrators control which actions appear for each role via `config.yaml`:

```yaml
services:
  - id: "user-service"
    name: "User Management"
    target: "localhost:50051"
    methods:
      - "user.v1.UserService/CreateUser"
      - "user.v1.UserService/ListUsers"
    roles: ["admin"]

  - id: "content-service"
    name: "Content Moderation"
    target: "localhost:50052"
    methods:
      - "content.v1.ContentService/ApproveArticle"
      - "content.v1.ContentService/RejectArticle"
    roles: ["admin", "moderator"]
```

### Role-Based Access

Users are assigned roles through your authentication system. The gateway reads the `X-User-Role` header and shows only the actions permitted for that role.

| Role | Sees |
|------|------|
| `admin` | All configured methods |
| `moderator` | Only methods assigned to moderator role |
| `viewer` | Read-only methods (list, get) |

## Form Annotations Reference

### Message-Level Options

Apply to your request message:

| Option | Description |
|--------|-------------|
| `title` | Form title displayed to users |
| `description` | Help text shown below the title |

### Field-Level Options

Apply to individual fields:

| Option | Description |
|--------|-------------|
| `label` | Human-readable field label |
| `hint` | Helper text explaining what to enter |
| `placeholder` | Example text shown in empty inputs |
| `hidden` | Hide field from form (for internal IDs, timestamps) |
| `collapsible` | Make nested sections collapsible |
| `order` | Control field display order |

## Response Display

When an action completes successfully, results are displayed in auser-friendly format:

- **Lists** are shown as tables with alternating row colors for readability
- **Objects** are displayed as labeled fields
- **Simple values** appear with clear formatting

## Running the Gateway

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `LOG_LEVEL` | INFO | Logging level |
| `HTTP_PORT` | 8080 | HTTP server port |
| `CONFIG_PATH` | config.yaml | Path to configuration file |
| `ALLOW_DEV_MODE` | false | Skip role checking for development. **Never enable in production.** |

### Development Mode

```bash
ALLOW_DEV_MODE=true go run .
```

This skips role requirements for local testing.

### Production

```bash
go build -o grpc-form .
./grpc-form
```

Ensure your authentication gateway sets the `X-User-Role` header.

## Example: Quick Start

1. Start a demo gRPC service with reflection:
   ```bash
   ENABLE_GRPC_REFLECTION=true go run ./cmd/demouser
   ```

2. Start the admin gateway:
   ```bash
   ALLOW_DEV_MODE=true go run .
   ```

3. Open http://localhost:8080 in your browser

4. Select "Create User" or "List Users" to see auto-generated forms

## Architecture

The gateway uses gRPC reflection to discover service schemas at runtime, meaning:

- No code generation when adding new methods
- Forms update automatically when proto files change
- Validation rules are enforced server-side using protovalidate
- RBAC is centralized in the gateway configuration