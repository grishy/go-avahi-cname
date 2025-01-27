# syntax=docker/dockerfile:1

# NOTE: I build images for multi-arch because goreleaser works strange with multi-stage builds
# Need a lot of additional configuration to build multi-arch images

# STAGE 1: building the executable
FROM docker.io/golang:1.23.5-alpine3.20 AS builder
WORKDIR /build

ARG VERSION
ARG COMMIT
ARG DATE

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 \
    go build \
    -ldflags="-w -s \
    -X main.version='${VERSION}' \
    -X main.commit='${COMMIT}' \
    -X main.date='${DATE}'" \
    -o /go-avahi-cname

# STAGE 2: build the container to run
FROM scratch
COPY --from=builder /go-avahi-cname /go-avahi-cname

EXPOSE 5353/udp

ENTRYPOINT ["/go-avahi-cname"]
CMD [ "subdomain" ]
