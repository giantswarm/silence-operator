# Testing Guide for Silence Operator

This document explains the testing infrastructure and practices for the Silence Operator project.

## Overview

The Silence Operator uses a comprehensive testing strategy that includes unit tests, integration tests, and end-to-end tests. The testing infrastructure is built on top of Kubebuilder's testing framework with enhancements for better developer experience.

## Testing Infrastructure

### Automatic Binary Detection

The test suite automatically detects and sets up the required Kubernetes testing binaries without requiring manual `KUBEBUILDER_ASSETS` configuration:

```go
func getKubeBuilderAssets() string {
    // Automatically detects k8s binaries in bin/k8s/ directory
    // Falls back to environment variable if needed
}
```

### Mock Alertmanager

The project includes a sophisticated mock Alertmanager HTTP server for testing:

- **File**: `internal/controller/testutils/mock_alertmanager.go`
- **Features**: 
  - Full CRUD operations for silences
  - Realistic HTTP responses
  - Configurable behavior for error scenarios
  - Thread-safe operation

### Test Environment Setup

The test suite automatically:
1. Detects available Kubernetes binaries
2. Starts etcd and kube-apiserver
3. Registers Custom Resource Definitions
4. Sets up test manager and controllers
5. Provides cleanup mechanisms

## Running Tests

### Prerequisites

Install required testing tools:
```bash
make install-tools
```

This installs:
- `ginkgo` - BDD testing framework
- `setup-envtest` - Kubernetes testing environment
- `golangci-lint` - Code quality checks
- `controller-gen` - Code generation

### Available Test Targets

#### Basic Test Commands

```bash
# Run all tests
make test
```

#### Enhanced Test Commands

```bash
# Run tests with verbose output 
make test GINKGO_ARGS="-v"

# Run tests without parallelism
make test GINKGO_ARGS="-p=false"

# Run specific test with focus
make test GINKGO_ARGS="-focus='Silence Controller'"

# Skip specific tests
make test GINKGO_ARGS="-skip='Should fail'"
```
### Test Configuration

#### Environment Variables

- `KUBEBUILDER_ASSETS` - Path to Kubernetes testing binaries (auto-detected if not set)
- `ENVTEST_K8S_VERSION` - Kubernetes version for envtest (defaults to k8s.io/api module version)

#### Test Flags

```bash
# Run specific test suites using Ginkgo args
make test GINKGO_ARGS="-focus='Controller'"

# Skip certain tests
make test GINKGO_ARGS="-skip='Integration'"

# Set Kubernetes version for envtest
make test ENVTEST_K8S_VERSION="1.29"
```

## Writing Tests

### Controller Tests Example

```go
var _ = Describe("Silence Controller", func() {
    var (
        ctx           context.Context
        cancel        context.CancelFunc
        mockAM        *testutils.MockAlertmanager
        reconciler    *SilenceReconciler
    )

    BeforeEach(func() {
        ctx, cancel = context.WithCancel(context.Background())
        
        // Setup mock Alertmanager
        mockAM = testutils.NewMockAlertmanager()
        mockAM.Start()
        
        // Create reconciler with mock
        reconciler = &SilenceReconciler{
            Client:            k8sClient,
            Scheme:            k8sManager.GetScheme(),
            AlertmanagerURL:   mockAM.URL,
        }
    })

    AfterEach(func() {
        if mockServer != nil {
            mockServer.Close()
        }
        cancel()
    })

    Context("When creating a Silence", func() {
        It("Should create silence in Alertmanager", func() {
            // Test implementation
        })
    })
})
```

### Service Layer Testing

The refactored architecture enables easier testing of business logic:

```go
// Test the service layer directly
func TestSilenceService_CreateOrUpdateSilence(t *testing.T) {
    // Setup mock AlertManager client
    mockClient := &MockAlertManagerClient{}
    service := NewSilenceService(mockClient)
    
    // Test business logic without Kubernetes dependencies
    err := service.CreateOrUpdateSilence(ctx, "test-comment", silence)
    assert.NoError(t, err)
    
    // Verify expected AlertManager operations
    mockClient.AssertCreateSilenceCalled(t, silence)
}
```

### Mock Alertmanager Usage

```go
// Create and start mock
mockAM := testutils.NewMockAlertmanager()
mockAM.Start()
defer mockAM.Stop()

// Configure responses
mockAM.SetResponse("/api/v2/silences", http.StatusOK, silenceList)

// Verify calls
Expect(mockAM.GetRequestCount("POST", "/api/v2/silences")).To(Equal(1))
```

## Coverage Reporting

### Coverage Collection

The project uses enhanced coverage collection that:
- Combines coverage from multiple test runs
- Excludes generated code from coverage calculations
- Provides detailed per-package breakdown
- Generates HTML reports for visualization

### Coverage Analysis

The test target automatically generates coverage information with Ginkgo:

```bash
# Run tests (includes coverage)
make test

# Generate HTML coverage report
go tool cover -html=coverprofile.out -o coverage.html

# View coverage summary
go tool cover -func=coverprofile.out
```

### Coverage Output

Coverage reports are saved to:
- `coverprofile.out` - Default coverage profile from Ginkgo via make test
- `coverage.html` - HTML visualization (if generated manually)

## Debugging Tests

### Verbose Output

```bash
# Show detailed Ginkgo output
make test GINKGO_ARGS="-v --trace"
```

### Test Environment Debugging

```bash
# Check tool versions  
./bin/setup-envtest version
./bin/ginkgo version
./bin/golangci-lint version

# Check envtest binary availability
./bin/setup-envtest list

# Verify tools are installed
make install-tools
```

### Common Issues

1. **Missing Kubernetes Binaries**
   ```bash
   # Solution: Install envtest binaries
   make setup-envtest
   ```

2. **CRD Registration Failures**
   ```bash
   # Solution: Regenerate CRDs
   make generate-crds
   ```

3. **Test Timeouts**
   ```bash
   # Solution: Increase timeout with Ginkgo args
   make test GINKGO_ARGS="-timeout=15m"
   ```

## Continuous Integration

### GitHub Actions Integration

The testing infrastructure integrates with GitHub Actions for:
- Automated test execution on PR creation
- Coverage reporting with trend analysis
- Compatibility testing across Kubernetes versions
- Security scanning with integrated results

### Quality Gates

Tests must pass the following quality gates:
- All unit and integration tests passing
- Minimum 40% code coverage
- No critical linting violations
- All generated code up to date
- Security scan with no high-severity issues

## Best Practices

### Test Organization

1. **Use descriptive test names** that explain the scenario being tested
2. **Group related tests** using Ginkgo's `Context` and `Describe` blocks
3. **Setup and cleanup** properly in `BeforeEach`/`AfterEach` hooks
4. **Use table-driven tests** for testing multiple scenarios

### Mocking Strategy

1. **Mock external dependencies** like Alertmanager API calls
2. **Use real Kubernetes API** for testing controller logic
3. **Prefer HTTP mocks** over interface mocks for external services
4. **Make mocks configurable** for different test scenarios

### Coverage Guidelines

1. **Aim for >40% overall coverage** with focus on critical paths
2. **Exclude generated code** from coverage calculations
3. **Test error paths** and edge cases
4. **Document untested code** with justification

### Performance Considerations

1. **Use parallel test execution** where safe
2. **Minimize test environment setup** time
3. **Share test fixtures** when possible
4. **Clean up resources** to prevent test interference

## Tools and Dependencies

### Core Testing Tools

- **Ginkgo**: BDD testing framework for readable tests
- **Gomega**: Assertion library with fluent interface
- **envtest**: Kubernetes API server for integration testing
- **controller-runtime**: Testing utilities for controllers

### Code Quality Tools

- **golangci-lint**: Comprehensive Go linting
- **gosec**: Security vulnerability scanning
- **gocyclo**: Cyclomatic complexity analysis
- **staticcheck**: Advanced static analysis

### Coverage Tools

- **go tool cover**: Built-in coverage analysis
- **gocov**: Enhanced coverage reporting
- **gocov-html**: HTML coverage visualization

## Troubleshooting

### Common Test Failures

1. **"no matches for kind" errors**: CRDs not properly registered
2. **"connection refused" errors**: envtest not started correctly
3. **"timeout" errors**: Tests taking too long, increase timeout
4. **"resource not found" errors**: Missing RBAC permissions in test

### Debug Commands

```bash
# Check envtest binary availability
./bin/setup-envtest list

# Verify CRD installation
kubectl get crd --context=envtest

# Check controller logs in tests
make test GINKGO_ARGS="-v" 2>&1 | grep "controller"
```

For more specific troubleshooting, refer to the individual test files and the Kubebuilder documentation.
