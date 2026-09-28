FROM golang:1.26-alpine AS build

# go.mod pins the language version; if it moves ahead of this image, fetch the
# toolchain it asks for rather than failing an operator's first build outright.
ENV GOTOOLCHAIN=auto

WORKDIR /app

RUN apk add --no-cache ca-certificates git

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o nebula ./cmd/nebula

FROM alpine:3.20

WORKDIR /app

RUN apk add --no-cache ca-certificates && update-ca-certificates

COPY --from=build /app/nebula /app/nebula

ENV DATA_DIR=/data
EXPOSE 4010

CMD ["/app/nebula"]
