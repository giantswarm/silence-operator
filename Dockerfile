# Use distroless as minimal base image to package the silence-operator binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM gcr.io/distroless/static:nonroot
ARG TARGETARCH
WORKDIR /

COPY silence-operator-linux-${TARGETARCH} /silence-operator

USER 65532:65532

ENTRYPOINT ["/silence-operator"]
