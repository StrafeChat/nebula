# Dockerfile for nebula
FROM golang:1.23

WORKDIR /app

COPY . .

RUN go mod tidy && \
    CGO_ENABLED=0 go build -o nebula ./main.go

EXPOSE 4002

CMD ["./nebula"]
