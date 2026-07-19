# syntax=docker/dockerfile:1

ARG GO_VERSION=1.25

FROM node:22-alpine AS frontend-build
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci --ignore-scripts
COPY . .
RUN npm run build

FROM golang:${GO_VERSION}-alpine AS backend-build
WORKDIR /src
COPY backend-go/go.mod backend-go/go.sum ./
RUN go mod download
COPY backend-go ./
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/halowebui ./cmd/halowebui

FROM gcr.io/distroless/static-debian12:nonroot
ARG BUILD_HASH=dev
ENV PORT=8080 \
    HOST=0.0.0.0 \
    FRONTEND_DIR=/app/build \
    DATA_DIR=/app/data \
    HALO_GO_MEMORY_LIMIT_MIB=48 \
    WEBUI_NAME=HaloWebUI \
    WEBUI_BUILD_VERSION=${BUILD_HASH} \
    ENABLE_WEBSOCKET_SUPPORT=false \
    ENABLE_LOCAL_MODEL_RUNTIME=false
WORKDIR /app
COPY --from=backend-build /out/halowebui /app/halowebui
COPY --from=frontend-build /app/build /app/build
VOLUME ["/app/data"]
EXPOSE 8080
HEALTHCHECK --interval=10s --timeout=2s --start-period=10s --retries=3 CMD ["/app/halowebui", "healthcheck"]
ENTRYPOINT ["/app/halowebui"]
