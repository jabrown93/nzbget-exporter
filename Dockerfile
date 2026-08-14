# Static CGO_ENABLED=0 cross-compile on the DHI Go toolchain, scratch runtime,
# nonroot.
FROM --platform=$BUILDPLATFORM dhi.io/golang:1.26.6-dev@sha256:dc7d057503c33ca36ab3e845d2f4f180700c3d358ed39ec783c10094eb7b21b0 AS builder

ARG TARGETOS
ARG TARGETARCH

WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags='-w -s' -o /out/nzbget_exporter .

FROM scratch

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=builder /out/nzbget_exporter /nzbget_exporter

USER 65532:65532
EXPOSE 9452
ENTRYPOINT ["/nzbget_exporter"]
