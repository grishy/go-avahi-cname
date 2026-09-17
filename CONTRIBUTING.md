## Development Setup

```bash
git clone https://github.com/grishy/go-avahi-cname.git
cd go-avahi-cname
go mod download
go build -o go-avahi-cname .
```

## Before Committing

Run formatting, linting, and tests:

```bash
golangci-lint fmt ./...
golangci-lint run ./...
go test -race -shuffle=on -vet=all -failfast ./...
```

All three should pass with no errors.

## Testing Locally

You need a Linux machine with Avahi daemon running.

```bash
# Run with debug logging
./go-avahi-cname --debug subdomain

# From another terminal or device
ping anything.yourhostname.local
```

### Isolated integration tests

These tests use real Avahi and D-Bus daemons in Docker, including multicast DNS
and shutdown checks for both CLI modes. They are excluded from `go test ./...`
by the `integration` build tag.

From the repository root, with Docker running:

```sh
# Use GOARCH=amd64 instead if your Docker engine runs on amd64.
GOTOOLCHAIN=go1.27.1 GOOS=linux GOARCH=arm64 CGO_ENABLED=0 \
  go test -tags=integration -c -o /tmp/cname-publisher.test ./avahi
GOTOOLCHAIN=go1.27.1 GOOS=linux GOARCH=arm64 CGO_ENABLED=0 \
  go build -o /tmp/cname-cli .

docker build -t go-avahi-cname-integration:local tests/integration
docker network create cname-integration
docker run --name cname-integration --network cname-integration \
  --hostname cname-test \
  -e AVAHI_TEST_BINARY=/go-avahi-cname \
  -v /tmp/cname-publisher.test:/publisher.test:ro \
  -v /tmp/cname-cli:/go-avahi-cname:ro \
  go-avahi-cname-integration:local

# Collect diagnostics even if the tests fail. Use a fresh destination per run.
docker cp cname-integration:/results /tmp/cname-integration-results
docker rm cname-integration
docker network rm cname-integration
docker image rm go-avahi-cname-integration:local
rm /tmp/cname-publisher.test /tmp/cname-cli
```

Use the dedicated bridge network above: Docker `--internal` omits the route
needed for multicast queries. Do not use host networking or mount the host's
D-Bus socket. No host ports or writable repository mounts are needed.

## Code Style

This project uses [golangci-lint](https://golangci-lint.run/) with a strict
config (see `.golangci.yml`). The linter handles formatting via `goimports`,
`gofumpt`, and `golines`.

A few principles:

- Meaningful variable names — no abbreviations unless standard
- Comments explain "why," not "what"
- Keep functions focused and reasonably sized

## Submitting Changes

1. Fork the repository
2. Create a feature branch: `git checkout -b feature/your-feature`
3. Make changes, run the checks above
4. Push to your fork and open a Pull Request

## Reporting Issues

Please include:

- OS and version
- Avahi daemon version (`avahi-daemon --version`)
- Output with `--debug` flag
- Steps to reproduce

## Release Process (Maintainers)

1. Update version references in the project.

2. Commit and push the changes. Wait for all CI jobs, including the Avahi
   integration tests, to pass for that commit.

3. Create and push a tag on the checked commit:

   ```bash
   git tag -a v2.7.0 -m "Release v2.7.0"
   git push origin v2.7.0
   ```

4. GitHub Actions will:
   - Build binaries for all platforms
   - Create a GitHub release
   - Push Docker images to GHCR

## License

By contributing, you agree that your contributions will be licensed under the MIT License.
