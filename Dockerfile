FROM golang:1.25-alpine AS build
WORKDIR /go/src/smolkafka
COPY . .
# Disable cgo compiler
RUN CGO_ENABLED=0 go build -o /go/bin/smolkafka ./cmd/smolkafka
RUN GRPC_HEALTH_PROBE_VERSION=v0.3.2 && \
  wget -q0/go/bin/grpc_health_probe \
  https://github.com/grpc-ecosystem/grpc-health-probe/releases/download/\
  ${GRPC_HEALTH_PROBE_VERSION}/grpc_health_probe-linux-amd64 && \
  chmod +x /go/bin/grpc_health_probe
  

# Smallest Docker image
FROM scratch
COPY --from=build /go/bin/smolkafka /bin/smolkafka
COPY --from=build /go/bin/grpc_health_probe /bin/grpc_health_probe
ENTRYPOINT ["/bin/smolkafka"]
