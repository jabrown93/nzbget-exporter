# Static CGO_ENABLED=0 cross-compile on the DHI Go toolchain, scratch runtime,
# nonroot.
FROM --platform=$BUILDPLATFORM dhi.io/golang:1.26.5-dev@sha256:2a26f82a4cddde8c6ababc5e94816a23ea1165f7a2c6548cf6e09f813bc6faf3 AS builder

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
