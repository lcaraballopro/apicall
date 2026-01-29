# AGENTS.md - Development Guidelines for Apicall

## Project Overview
Apicall is a high-performance Go microservice for automated IVR calling campaigns. It's an independent system designed to handle thousands of concurrent calls with robust retry mechanisms and database optimization.

## Build, Test, and Development Commands

### Core Commands
```bash
# Build the application
make build

# Build for production (optimized)
make build-prod

# Run the application
make run

# Run all tests
make test

# Run tests for a specific package
go test -v ./internal/database

# Run a single test function
go test -v ./internal/database -run TestCreateProyecto

# Clean build artifacts
make clean

# Download and tidy dependencies
make deps

# Install binaries to /usr/local/bin
make install
```

### Development Workflow
```bash
# Ensure dependencies are up to date
go mod tidy
go mod download

# Run with specific config file
APICALL_CONFIG=./configs/apicall.yaml ./bin/apicall start

# Build CLI tool
go build -o bin/apicall-cli ./cmd/apicall-cli

# Lint and format (if available)
go fmt ./...
go vet ./...
```

## Code Style Guidelines

### Go Conventions
- Follow standard Go formatting (`go fmt`)
- Use `gofmt` for code formatting
- Package names should be short, lowercase, single words
- Use camelCase for variable and function names
- Use PascalCase for exported types and functions
- Use underscore for private/internal variables when needed

### File Structure
```
cmd/           - Main applications (CLI entry points)
internal/      - Private application code
  api/         - REST API server
  database/    - Database models and operations
  auth/        - Authentication (JWT)
  config/      - Configuration management
  fastagi/     - FastAGI server implementation
  ami/         - Asterisk Manager Interface client
  asterisk/    - Asterisk integration
  provisioning/ - System provisioning
  smartcid/    - Smart Caller ID logic
  sysadmin/    - System administration utilities
migrations/    - Database migration scripts
configs/       - Configuration files
scripts/       - Utility scripts
tools/         - Build and development tools
web/           - Web dashboard assets
```

### Import Organization
```go
import (
    // Standard library
    "fmt"
    "log"
    "os"
    
    // Third-party libraries
    "github.com/gin-gonic/gin"
    "gopkg.in/yaml.v3"
    
    // Internal packages
    "apicall/internal/config"
    "apicall/internal/database"
)
```

### Error Handling
- Always handle errors explicitly
- Use wrapped errors with context: `fmt.Errorf("operation failed: %w", err)`
- Log errors with context: `[Module] Error description: %v`
- Return errors from functions, don't panic unless unrecoverable
- Use structured error messages

### Naming Conventions
- **Interfaces**: Single method interfaces named with -er suffix (e.g., `Writer`, `Reader`)
- **Structs**: PascalCase, descriptive names (e.g., `Proyecto`, `CallLog`, `Troncal`)
- **Functions**: camelCase, verb-noun pattern (e.g., `CreateProyecto`, `ListTroncales`)
- **Constants**: SCREAMING_SNAKE_CASE for exported constants
- **Variables**: camelCase, descriptive names
- **File names**: snake_case.go (e.g., `database.go`, `config.go`)

### Database Patterns
- Use the `db` struct tag for database column mapping
- Use `json` struct tag for API serialization
- Always include `id`, `created_at`, `updated_at` where applicable
- Use pointer types for nullable fields (`*string`, `*int`)
- Implement repository pattern for data access

### API Patterns
- Use RESTful endpoints
- Implement JWT authentication
- Return JSON responses with consistent structure
- Use appropriate HTTP status codes
- Validate input parameters
- Handle errors gracefully with proper HTTP status codes

### Configuration
- Use YAML configuration files
- Support environment variable overrides with `APICALL_` prefix
- Provide sensible defaults
- Use struct tags for configuration mapping
- Validate configuration on load

### Logging
- Use structured logging with module prefixes: `[Module] Message`
- Include context information in log messages
- Use appropriate log levels (INFO, WARN, ERROR)
- Avoid logging sensitive information (passwords, tokens)

### Testing
- Write unit tests for business logic
- Use table-driven tests for multiple scenarios
- Mock external dependencies
- Test both success and error cases
- Aim for good test coverage of critical paths

### Performance Considerations
- Use connection pooling for database
- Implement batching for bulk operations
- Use goroutines carefully with proper synchronization
- Consider memory usage in long-running processes
- Implement rate limiting where appropriate

### Security
- Never hardcode credentials
- Use environment variables for secrets
- Implement proper authentication and authorization
- Validate all input data
- Use parameterized queries to prevent SQL injection
- Implement rate limiting for API endpoints

### Concurrency
- Use channels for communication between goroutines
- Implement proper mutex usage for shared state
- Use context for cancellation and timeouts
- Be aware of race conditions
- Test concurrent scenarios

### Code Comments
- Comment complex logic or algorithms
- Document public APIs with proper godoc format
- Explain configuration options
- Comment non-obvious business logic
- Keep comments concise and relevant