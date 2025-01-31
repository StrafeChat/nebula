# Dockerfile for nebula
FROM golang:1.23

WORKDIR /app

COPY . .

RUN go mod tidy && \
    CGO_ENABLED=0 go build -o nebula ./src/main.go

EXPOSE 443

CMD ["./nebula"]
