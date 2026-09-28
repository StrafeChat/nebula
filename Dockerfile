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

# Fetch the emoji artwork sets nebula seeds into the store so the client renders emoji from
# this instance, not a third-party CDN (scripts/fetch-emoji.sh; licences credited in the web
# client's CREDITS.md). Set --build-arg FETCH_EMOJI=0 to build offline; nebula then simply
# serves no bundled emoji and clients fall back to the system font.
ARG FETCH_EMOJI=1
RUN if [ "$FETCH_EMOJI" = "1" ]; then \
      apk add --no-cache bash curl tar && bash scripts/fetch-emoji.sh /app/emoji; \
    else mkdir -p /app/emoji; fi

FROM alpine:3.20

WORKDIR /app

RUN apk add --no-cache ca-certificates && update-ca-certificates

COPY --from=build /app/nebula /app/nebula
COPY --from=build /app/emoji /app/emoji

ENV DATA_DIR=/data \
    EMOJI_ASSETS_DIR=/app/emoji
EXPOSE 4010

CMD ["/app/nebula"]
