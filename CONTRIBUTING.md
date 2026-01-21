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

2. Create and push a tag:

   ```bash
   git tag -a v2.5.0 -m "Release v2.5.0"
   git push origin tag v2.5.0
   ```

3. GitHub Actions will:
   - Build binaries for all platforms
   - Create a GitHub release
   - Push Docker images to GHCR

## License

By contributing, you agree that your contributions will be licensed under the MIT License.
