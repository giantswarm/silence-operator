# Use a build stage to select the correct binary based on TARGETPLATFORM
FROM alpine AS binary-selector
ARG TARGETPLATFORM
COPY silence-operator-* /binaries/
RUN arch="${TARGETPLATFORM#linux/}" && cp "/binaries/silence-operator-linux-${arch}" /silence-operator

# Use distroless as minimal base image to package the silence-operator binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=binary-selector /silence-operator /silence-operator
USER 65532:65532
ENTRYPOINT ["/silence-operator"]
