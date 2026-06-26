# syntax=docker/dockerfile:1

# 1) Build the SPA.
FROM node:22-alpine AS web
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
# gen/ts is outside web/ but referenced via @gen alias (../gen/ts).
COPY gen/ts /src/gen/ts
RUN npm run build

# 2) Build the static Go binary with the SPA embedded.
FROM golang:1.25 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /src/web/dist ./web/dist
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" \
    -o /out/nebari-frames-server ./backend/cmd/server
RUN mkdir -p /data

# 3) Minimal nonroot runtime.
FROM gcr.io/distroless/static:nonroot
COPY --from=build /out/nebari-frames-server /nebari-frames-server
COPY --from=build --chown=65532:65532 /data /data
ENV DB_PATH=/data/nebari-frames.db
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/nebari-frames-server"]
