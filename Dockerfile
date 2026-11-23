# Static CGO_ENABLED=0 cross-compile on the DHI Go toolchain, scratch runtime,
# nonroot.
FROM --platform=$BUILDPLATFORM dhi.io/golang:1.27.4-alpine-dev@sha256:f5765dcea1354da7d6b6e12eb84949fed6336dc9c022edc49af932be879f71c0 AS builder

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
