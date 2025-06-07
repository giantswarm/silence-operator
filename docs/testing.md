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

# Run V2 controller tests specifically
make test GINKGO_ARGS="-focus='SilenceV2 Controller'"

# Run config package tests
go test ./pkg/config/ -v

# Run all controller tests
KUBEBUILDER_ASSETS="$(./bin/setup-envtest use 1.31.0 -p path)" ./bin/ginkgo run -v ./internal/controller/
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

### Controller Tests

The project includes comprehensive tests for both API versions:

#### V1 Controller Tests (`silence_controller_test.go`)
Tests for the `monitoring.giantswarm.io/v1alpha1` API:
- Basic reconciliation functionality
- Resource creation and cleanup
- Alertmanager integration

#### V2 Controller Tests (`silence_v2_controller_test.go`)
Tests for the `observability.giantswarm.io/v1alpha2` API:
- Basic reconciliation functionality
- Finalizer handling and deletion
- API conversion between v1alpha2 and v1alpha1
- Enhanced error handling

```go
var _ = Describe("SilenceV2 Controller", func() {
    Context("When reconciling a resource", func() {
        var mockServer *testutils.MockAlertmanagerServer
        var mockAlertmanager *alertmanager.Alertmanager

        BeforeEach(func() {
            // Set up mock Alertmanager server
            mockServer = testutils.NewMockAlertmanagerServer()
            mockAlertmanager, err = mockServer.GetAlertmanager()
            Expect(err).NotTo(HaveOccurred())
        })

        AfterEach(func() {
            if mockServer != nil {
                mockServer.Close()
            }
        })

        It("should successfully reconcile the resource", func() {
            controllerReconciler := &SilenceV2Reconciler{
                Client:       k8sClient,
                Scheme:       k8sClient.Scheme(),
                Alertmanager: mockAlertmanager,
            }
            // Test implementation
        })

        It("should handle deletion with finalizer", func() {
            // Tests finalizer addition and removal during resource deletion
        })

        It("should convert v1alpha2 to v1alpha1 format correctly", func() {
            // Tests API conversion functionality
        })
    })
})
```

### Config Package Tests

The `pkg/config` package includes comprehensive tests for configuration parsing:

```go
func TestParseSilenceSelector(t *testing.T) {
    tests := []struct {
        name           string
        silenceSelector string
        expectError    bool
        expectNil      bool
    }{
        {"empty selector returns nil", "", false, true},
        {"valid single label selector", "app=nginx", false, false},
        {"valid multiple label selector", "app=nginx,env=prod", false, false},
        {"invalid selector returns error", "invalid=", true, false},
    }
    // Test implementation covers various selector scenarios
}
```

Coverage includes:
- Empty selector handling
- Valid single and multiple label selectors
- Complex selectors with set-based requirements
- Invalid selector error handling
- Label matching validation

#### Key Test Scenarios for ParseSilenceSelector

The helper function `ParseSilenceSelector` is thoroughly tested with:

1. **Empty selectors**: Returns `nil` without error for empty strings
2. **Single label selectors**: `"app=nginx"` - basic equality matching
3. **Multiple label selectors**: `"app=nginx,env=prod"` - comma-separated labels
4. **Set-based selectors**: `"env notin (test,staging)"` - advanced matching
5. **Invalid syntax**: Malformed selectors return proper error messages
6. **Label validation**: Ensures selectors can match expected label sets

This testing ensures robust configuration parsing for silence selector functionality extracted from `main.go` into reusable helper functions.

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

## Recent Testing Improvements

### V2 API Controller Testing (June 2025)

Added comprehensive test coverage for the new `observability.giantswarm.io/v1alpha2` API:
- **Basic reconciliation testing**: Validates V2 controller functionality
- **Finalizer management testing**: Ensures proper resource cleanup
- **API conversion testing**: Validates conversion between v1alpha2 and v1alpha1 formats
- **Resource isolation**: Each test creates its own resources to avoid conflicts

### Configuration Testing Enhancement

Extracted silence selector parsing logic from `main.go` into `pkg/config/config.go` with comprehensive testing:
- **ParseSilenceSelector function**: Helper function with 7 test scenarios
- **Error handling**: Proper error wrapping and context
- **Label selector validation**: Kubernetes-compatible selector parsing
- **Edge case coverage**: Empty selectors, malformed syntax, complex requirements

### Testing Infrastructure Updates

- **Enhanced mock Alertmanager**: Improved testutils with better error simulation
- **Automatic binary detection**: Tests automatically find required K8s binaries
- **Parallel test execution**: Safe parallel execution where applicable
- **Better resource cleanup**: Improved test isolation and cleanup

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

#### Current Test Structure

```
internal/controller/
├── suite_test.go              # Test suite setup and configuration
├── silence_controller_test.go # V1 API tests (monitoring.giantswarm.io/v1alpha1)
├── silence_v2_controller_test.go # V2 API tests (observability.giantswarm.io/v1alpha2)
└── testutils/                 # Mock utilities and test helpers

pkg/config/
└── config_test.go            # Configuration parsing tests
```

Each test file follows a consistent pattern:
- **Suite setup**: Shared test environment initialization
- **Mock setup**: Alertmanager server mocking for external dependencies  
- **Resource lifecycle**: Creation, reconciliation, and cleanup testing
- **Error scenarios**: Validation of error handling and edge cases

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
