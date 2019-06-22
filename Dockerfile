from golang:1.12 as builder
add . /src
workdir /src
env GO111MODULE on
run go build -mod=vendor .

from alpine:latest as certs
run apk --update add ca-certificates

from scratch
env DOCKER_API_VERSION 1.38
copy --from=certs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
copy --from=builder /src/docker-remote-logs /
entrypoint ["/docker-remote-logs"]
