# syntax=docker/dockerfile:1

ARG GO_VERSION=1.24
ARG NODE_VERSION=22

FROM golang:${GO_VERSION}-bookworm AS builder

ARG NODE_VERSION
ENV CGO_ENABLED=0
WORKDIR /src

RUN apt-get update \
 && apt-get install -y --no-install-recommends curl ca-certificates gnupg make \
 && curl -fsSL "https://deb.nodesource.com/setup_${NODE_VERSION}.x" | bash - \
 && apt-get install -y --no-install-recommends nodejs \
 && rm -rf /var/lib/apt/lists/*

COPY go.mod go.sum ./
RUN go mod download

COPY package.json package-lock.json ./
RUN npm ci

COPY . .

ARG CONFIG_PATH=config.example.json
RUN make build CONFIG=${CONFIG_PATH} BINARY=/out/landing

FROM gcr.io/distroless/base-debian12 AS runner

WORKDIR /app
COPY --from=builder /out/landing ./landing
ENV PORT=8080
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["./landing"]
