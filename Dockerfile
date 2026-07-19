# syntax=docker/dockerfile:1

ARG GO_VERSION=1.25
ARG NODE_VERSION=22

FROM node:${NODE_VERSION}-alpine AS frontend-build
ARG BUILD_HASH=dev
WORKDIR /app
COPY package.json package-lock.json .npmrc ./
RUN npm ci --ignore-scripts
COPY src ./src
COPY static ./static
COPY scripts ./scripts
COPY CHANGELOG.md postcss.config.js svelte.config.js tailwind.config.js tsconfig.json vite.config.ts ./
ENV APP_BUILD_HASH=${BUILD_HASH} ENABLE_PYODIDE=false VITE_SOURCEMAP=false
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
