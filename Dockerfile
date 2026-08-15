# Stage 1: Build web frontend assets
FROM node:22-alpine AS web-builder

WORKDIR /app/web

COPY web/package.json web/package-lock.json* ./
RUN npm install

COPY web/ ./
RUN npm run build

# Stage 2: Build Go backend binary
FROM golang:1.25-alpine AS go-builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . ./
# Overwrite/ensure web/dist is populated from Stage 1
COPY --from=web-builder /app/web/dist ./web/dist

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /tour-map .

# Stage 3: Minimal production runtime image
FROM alpine:3.21

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

COPY --from=go-builder /tour-map /app/tour-map

EXPOSE 8080

CMD ["/app/tour-map"]
