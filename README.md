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
