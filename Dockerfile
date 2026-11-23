FROM golang:1.25.1-alpine AS builder

ARG TARGETARCH=arm64
ARG HTTP_PROXY
ARG HTTPS_PROXY
ARG ALL_PROXY
ARG NO_PROXY

WORKDIR /src
COPY go.mod go.sum ./
RUN export HTTP_PROXY="$HTTP_PROXY" HTTPS_PROXY="$HTTPS_PROXY" ALL_PROXY="$ALL_PROXY" NO_PROXY="$NO_PROXY" \
    && go mod download
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 GOOS=linux GOARCH="$TARGETARCH" go build -trimpath -o /out/shareo ./cmd/server

FROM alpine:3.22

RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=builder /out/shareo /app/shareo
COPY web /app/web
COPY deploy/config.orbstack.yaml /app/config.yaml

ENV SHAREO_CONFIG=/app/config.yaml
EXPOSE 8080
CMD ["/app/shareo"]
