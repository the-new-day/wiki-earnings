FROM golang:1.26-alpine AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download

COPY . .
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/app ./cmd/bot

# ca-certificates is not optional: the app talks to the wiki over HTTPS.
FROM alpine:3.21 AS runtime
RUN apk add --no-cache ca-certificates tzdata \
    && adduser -D -u 10001 app
USER app

FROM runtime AS app
COPY --from=build /out/app /usr/local/bin/app
ENTRYPOINT ["app"]
