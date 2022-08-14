from golang:1.19 as builder
workdir /src
add go.mod go.sum /src/
run go mod download
add . /src
run go build .

from alpine:latest as certs
run apk --update add ca-certificates

from gcr.io/distroless/base
env DOCKER_API_VERSION 1.38
copy --from=certs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
copy --from=builder /src/docker-remote-logs /docker-remote-logs
entrypoint ["/docker-remote-logs"]
cmd ["agent"]
