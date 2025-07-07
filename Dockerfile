# Stage 1: copy precompiled binaries
FROM alpine AS binary-selector
ARG TARGETPLATFORM
COPY silence-operator-* /binaries/

# Set the target binary path using TARGETPLATFORM
FROM gcr.io/distroless/static:nonroot
ARG TARGETPLATFORM
WORKDIR /
COPY --from=binary-selector /binaries/silence-operator-linux-${TARGETPLATFORM#linux/} /silence-operator
USER 65532:65532
ENTRYPOINT ["/silence-operator"]
