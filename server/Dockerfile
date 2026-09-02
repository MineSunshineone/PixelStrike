# Build stage
FROM golang:1.25-alpine AS builder
WORKDIR /app

# Copy module definitions and pre-download dependencies
COPY server/go.mod server/go.sum ./server/
WORKDIR /app/server
RUN go mod download || (sleep 2 && go mod download) || (sleep 5 && go mod download)

# Copy source and map definition
WORKDIR /app
COPY map.json ./map.json
COPY server ./server
WORKDIR /app/server
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /app/pixelstrike .

# Final minimal runtime container
FROM alpine:3.20
RUN apk --no-cache add ca-certificates tzdata
WORKDIR /app

COPY --from=builder /app/pixelstrike ./pixelstrike
COPY --from=builder /app/map.json ./map.json

ENV PORT=8080
ENV DB_PATH=/data/stats.db
ENV MAP_PATH=/app/map.json

VOLUME ["/data"]
EXPOSE 8080

ENTRYPOINT ["/app/pixelstrike"]
