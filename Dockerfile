FROM golang:1.23.9-alpine AS builder
RUN apk update && apk add --no-cache \
    build-base \
    git \
    protobuf-dev
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
RUN go mod verify
RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
RUN go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
COPY . .
RUN go generate ./...
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/subpub cmd/main.go

FROM alpine:latest
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=builder /app/subpub /app/subpub
COPY config.yaml /app/config.yaml
ENTRYPOINT ["/app/subpub"]
