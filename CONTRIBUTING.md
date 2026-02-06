# Contributing to Itchan

Thanks for your interest in contributing! This guide will help you get started.

## Getting Started

1. Fork the repository
2. Clone your fork: `git clone https://github.com/YOUR_USERNAME/itchan.git`
3. Set up the development environment (see [README.md](README.md#local-development))
4. Create a feature branch: `git checkout -b feature/your-feature`

## Development Setup

```bash
# Start PostgreSQL and configure config/private.yaml
cp config/private.yaml.example config/private.yaml

# Run the backend
cd backend && go run cmd/itchan-api/main.go -config_folder ../config

# Run the frontend (separate terminal)
cd frontend && go run cmd/frontend/main.go -config_folder ../config
```

Or use Docker:
```bash
docker-compose up --build
```

## Architecture

Itchan follows a strict **three-layer architecture**:

```
Handler  -> HTTP routing, JSON I/O, validation
Service  -> Business logic, authentication, file processing
Storage  -> PostgreSQL operations, queries
```

Please maintain this separation in your contributions. Do not put business logic in handlers or SQL in services.

## Commit Messages

We follow the [Conventional Commits](https://www.conventionalcommits.org/) specification.

### Format

```
<type>(<scope>): <short summary>

<optional body>
```

### Types

| Type       | Description                                           |
|------------|-------------------------------------------------------|
| `feat`     | New feature                                           |
| `fix`      | Bug fix                                               |
| `refactor` | Code change that neither fixes a bug nor adds feature |
| `docs`     | Documentation only                                    |
| `test`     | Adding or fixing tests                                |
| `chore`    | Build, CI, dependencies, tooling                      |
| `perf`     | Performance improvement                               |
| `ci`       | CI/CD changes                                         |

### Scopes

Optional, but encouraged: `backend`, `frontend`, `shared`, `config`, `ci`.

### Examples

```
feat(backend): add thread pinning endpoint
fix(frontend): prevent XSS in markdown preview
refactor(shared): extract rate limiter into middleware
docs: update API endpoint documentation
chore(ci): switch deploy trigger to version tags
```

### Rules

- Subject line under 72 characters
- Use imperative mood: "add", not "added" or "adds"
- No period at the end of the subject line
- Body explains **why**, not **what** (the diff shows what)
- One logical change per commit

## Versioning

This project uses [Semantic Versioning](https://semver.org/). Releases are created by tagging commits on `main`:

```
v<MAJOR>.<MINOR>.<PATCH>
```

- **MAJOR** - breaking API or schema changes
- **MINOR** - new features, backwards compatible
- **PATCH** - bug fixes

## Making Changes

1. **Write tests** for your changes
2. **Run tests** before submitting:
   ```bash
   cd backend && go test ./...
   cd ../frontend && go test ./...
   ```
3. **Format your code**: `go fmt ./...`
4. **Keep commits focused** - one logical change per commit (see [Commit Messages](#commit-messages))

## Pull Request Process

1. Update documentation if your changes affect the API or configuration
2. Ensure all tests pass
3. Write a clear PR description explaining what and why
4. Link any related issues

## Code Style

- Follow standard Go conventions (`gofmt`)
- Keep functions focused and testable
- Use meaningful variable and function names
- Add comments only for complex logic

## Reporting Issues

- Use GitHub Issues for bug reports and feature requests
- For security vulnerabilities, see [SECURITY.md](SECURITY.md)
- Include reproduction steps for bugs

## License

By contributing, you agree that your contributions will be licensed under the MIT License.
