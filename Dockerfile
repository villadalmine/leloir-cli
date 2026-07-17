# leloir CLI — imagen mínima (binario estático en distroless).
# Multi-arch: build con `docker buildx build --platform linux/amd64,linux/arm64`.
#
#   docker build --build-arg VERSION=$(cat VERSION) -t leloir-cli .
#   docker run --rm -e LELOIR_SERVER=https://leloir.example.com \
#              -e LELOIR_API_KEY=lk_... leloir-cli inv list

FROM --platform=$BUILDPLATFORM golang:1.26 AS build
ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# Cross-compilación sin emulación (el binario es Go puro, CGO off).
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath \
    -ldflags "-s -w -X main.version=${VERSION}" \
    -o /out/leloir ./cmd/leloir

# distroless static = ~2MB de base + CA certs (para HTTPS a la API), non-root.
FROM gcr.io/distroless/static-debian12:nonroot
LABEL org.opencontainers.image.source=https://github.com/villadalmine/leloir-cli
LABEL org.opencontainers.image.description="Leloir CLI — pure REST client of the governance control plane"
COPY --from=build /out/leloir /usr/local/bin/leloir
ENTRYPOINT ["/usr/local/bin/leloir"]
