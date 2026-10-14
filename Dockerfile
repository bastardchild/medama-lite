# syntax = docker/dockerfile:1

# MedamaLite builds a single Go binary. No Node.js, Bun or package manager is
# involved: the tracker scripts are frozen in core/client/scripts and the
# dashboard templates are embedded in core/ui.

# The builder tag must be >= the `go` directive in core/go.mod (currently 1.26.4).
# GOTOOLCHAIN is deliberately left at its default so that, if the image ships an
# older 1.26.x patch, Go can still fetch the exact toolchain required by go.mod
# instead of failing the build.
ARG GO_VERSION=1.26

# ------------------------------------------------------------------ builder ---
FROM golang:${GO_VERSION}-bookworm AS build

ARG VERSION=development
ARG COMMIT_SHA=development

ENV CGO_ENABLED=1 \
	GOFLAGS=-buildvcs=false

# go-duckdb links a prebuilt static libduckdb, but cgo still needs a C and C++
# toolchain to compile the driver glue.
RUN apt-get update \
	&& apt-get install -y --no-install-recommends gcc g++ \
	&& rm -rf /var/lib/apt/lists/*

WORKDIR /src

# Cache the module graph separately from the source tree.
COPY core/go.mod core/go.sum ./core/
RUN --mount=type=cache,target=/go/pkg/mod \
	cd core && go mod download

COPY core ./core

RUN --mount=type=cache,target=/go/pkg/mod \
	--mount=type=cache,target=/root/.cache/go-build \
	cd core && go build \
		-o /out/medamalite \
		-ldflags "-s -w -X main.Version=${VERSION} -X main.Commit=${COMMIT_SHA}" \
		-trimpath \
		./cmd/

# ------------------------------------------------------------------ runtime ---
FROM debian:bookworm-slim AS runtime

LABEL org.opencontainers.image.title="MedamaLite" \
	org.opencontainers.image.description="Cookie-free, privacy-focused website analytics." \
	org.opencontainers.image.licenses="Apache-2.0"

# ca-certificates is required for AutoSSL / ACME; tzdata keeps the timezone to
# country mapping used by the tracker accurate.
RUN apt-get update \
	&& apt-get install -y --no-install-recommends ca-certificates tzdata \
	&& rm -rf /var/lib/apt/lists/*

# HOME must point at a directory that actually exists and is writable. The
# `nobody` account ships with HOME=/nonexistent, which makes DuckDB fail at
# startup with "Can't find the home directory". Pointing HOME at the data volume
# also keeps DuckDB's extension cache persistent across container restarts.
ENV PORT=8080 \
	APP_DATABASE_HOST=/app/data/medama.db \
	ANALYTICS_DATABASE_HOST=/app/data/medama_analytics.duckdb \
	HOME=/app/data

WORKDIR /app

COPY --from=build /out/medamalite /app/medamalite

# The DuckDB and SQLite files live on a mounted volume so analytics survive
# container restarts.
RUN mkdir -p /app/data && chown -R nobody:nogroup /app/data

USER nobody:nogroup

EXPOSE 8080
VOLUME ["/app/data"]

CMD ["/app/medamalite", "start", "-env"]
