FROM node:24-alpine AS client

WORKDIR /app/web
COPY web/package.json web/package-lock.json* ./
RUN npm ci
COPY web/ ./
RUN npm run build

FROM golang:1.25-alpine AS backend

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . ./
COPY --from=client /app/web/dist ./web/dist

ENV CGO_ENABLED=0
RUN go build -ldflags="-s -w" -o /app/tour-map .

FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata

COPY --from=backend /app/tour-map /app/tour-map

EXPOSE 8080

CMD ["/app/tour-map"]
