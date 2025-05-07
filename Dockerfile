# syntax=docker/dockerfile:1
ARG GO_VERSION=1.24.3
FROM --platform=$BUILDPLATFORM golang:${GO_VERSION} AS build
WORKDIR /src

# golang dependencies
RUN --mount=type=cache,target=/go/pkg/mod/ \
  --mount=type=bind,source=go.sum,target=go.sum \
  --mount=type=bind,source=go.mod,target=go.mod \
  go mod download -x

# Target go version
ARG TARGETARCH

# Build the application, using cache mount.

RUN --mount=type=cache,target=/go/pkg/mod/ \
  --mount=type=bind,target=. \
  CGO_ENABLED=0 GOARCH=$TARGETARCH go build -o /bin/server ./

# Final stage copy bin and install pre-requisites
FROM alpine:latest AS final
LABEL org.opencontainers.image.source="https://github.com/babbage88/infractl"

WORKDIR /app
RUN touch default.yaml
ARG UID=10001
RUN adduser \
  --disabled-password \
  --gecos "" \
  --home "/nonexistent" \
  --shell "/sbin/nologin" \
  --no-create-home \
  --uid "${UID}" \
  appuser

RUN chown -R appuser:appuser /app/
USER appuser

# Copy the executable from the "build" stage.
COPY --from=build /bin/server /app/

# Expose the port that the application listens on.
EXPOSE 8181

ENTRYPOINT [ "/app/server", "database", "db-helper-api", "--start-api=true" ]
