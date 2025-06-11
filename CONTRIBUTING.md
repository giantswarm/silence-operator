# Contributing Guidelines

## How to contribute to the silence-operator

Thank you for your interest in contributing to the silence-operator! This guide will help you get started.

### Development Prerequisites

- Go 1.21+
- Kubernetes 1.25+
- Docker
- kubectl
- Helm 3+

### Setting up the Development Environment

1. Fork this repository and clone your fork
2. Install development tools:
   ```bash
   make install-tools
   ```
3. Generate code and manifests:
   ```bash
   make generate-all-with-helm
   ```

### Making Changes

1. **Create a feature branch** from `main`
2. **Make your changes** following the coding standards
3. **Write tests** for new functionality (both unit and integration tests)
4. **Update documentation** as needed
5. **Test your changes**:
   ```bash
   make test
   make lint
   ```
6. **Bump the chart version** for every change (see `helm/silence-operator/Chart.yaml`)
7. **Update CHANGELOG.md** with your changes

### API Version Considerations

The project supports two API versions:
- **v1alpha1**: Legacy cluster-scoped API (deprecated but maintained)
- **v1alpha2**: New namespace-scoped API (recommended for new features)

When adding features:
- Implement in v1alpha2 first
- Consider backward compatibility with v1alpha1 if applicable
- Update migration documentation if the change affects migration

### Testing Requirements

- Write tests for both API versions where applicable
- Include tests for error scenarios
- Ensure coverage remains above 40%
- Test migration scenarios if your changes affect both APIs

### Code Style

- Follow standard Go conventions
- Use meaningful variable and function names
- Add appropriate comments for public APIs
- Keep functions focused and testable

### Pull Request Process

1. Ensure all tests pass and linting is clean
2. Update documentation for any user-facing changes
3. Include a clear description of what your PR does
4. Reference any related issues
5. Be prepared to address review feedback

### Release Process

Releases are handled by the maintainers. After your PR is merged:
1. The CHANGELOG.md will be updated
2. A new version will be tagged
3. Container images will be built and pushed
4. Helm charts will be published

For more details, see our [Security Policy](SECURITY.md) for security-related contributions.
