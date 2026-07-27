FROM golang:1.25.1-alpine AS builder

ARG HTTP_PROXY
ARG HTTPS_PROXY
ARG ALL_PROXY
ARG NO_PROXY

WORKDIR /src
COPY go.mod go.sum ./
RUN export HTTP_PROXY="$HTTP_PROXY" HTTPS_PROXY="$HTTPS_PROXY" ALL_PROXY="$ALL_PROXY" NO_PROXY="$NO_PROXY" \
    && go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -o /out/shareo ./cmd/server

FROM alpine:3.22

RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=builder /out/shareo /app/shareo
COPY web /app/web
COPY deploy/config.docker.yaml /app/config.yaml

ENV SHAREO_CONFIG=/app/config.yaml
EXPOSE 8080
CMD ["/app/shareo"]
