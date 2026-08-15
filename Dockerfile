FROM golang:1.25

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . ./

RUN GOOS=linux go build -o /tour-map

EXPOSE 8080

# Run
CMD ["/tour-map"]
