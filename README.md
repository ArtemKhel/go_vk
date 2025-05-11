# SubPub gRPC Service

Test task for VK internship on "Go developer" position.

This service implements a publish-subscribe (PubSub) model in Go using gRPC.
Clients can subscribe to events by key and receive notifications when others publish events with the same key.

### Running

With docker-compose:

```shell
docker-compose up --build
```

or manually:
```shell
# REQUIRES a `protoc` 
# brew install protobuf
# apt install -y protobuf-compiler
# or whatever
go install google.golang.org/protobuf/cmd/protoc-gen-go
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc
go mod tidy
go generate ./...
go run ./cmd/main.go
```

Example query with `grpcurl`:
```shell
grpcurl -plaintext -d '{"key": "weather"}' localhost:8080 grpc.PubSub.Subscribe
grpcurl -plaintext -d '{"key": "weather", "data": "Sunny!"}' localhost:8080 grpc.PubSub.Publish
```

Running tests:
```shell
go test ./...
```

### Configuration

Config values are sourced from (in order of importance):

- env (i.e. `SUBPUB_LOG_LEVEL="debug"`)
- config.yaml
- hardcoded values

`./config.yaml` contains an example of configuration and additional value description

### Patterns

- Dependency Injection – components such as SubPub and the logger are injected into the service.
- Graceful Shutdown – the service handles termination signals and shuts down properly.
- Configuration Management – the configuration can defined via an env vars or YAML file.
- Structured Logging
