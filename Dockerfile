# Static CGO_ENABLED=0 cross-compile on the DHI Go toolchain, scratch runtime,
# nonroot.
FROM --platform=$BUILDPLATFORM dhi.io/golang:1.26.7-dev@sha256:05ecf8176801958c7b2afaf2f97923556124c79176a23611d80bf5000e31a028 AS builder

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
