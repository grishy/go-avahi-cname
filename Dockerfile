# syntax=docker/dockerfile:1

# STAGE 1: building the executable
FROM docker.io/golang:1.20-alpine as builder
WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

ENV CGO_ENABLED=0
COPY . .
RUN go build -ldflags="-w -s" -o /go-avahi-cname


# STAGE 2: build the container to run
FROM scratch
COPY --from=builder /go-avahi-cname /go-avahi-cname
ENTRYPOINT ["/go-avahi-cname"]