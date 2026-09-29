FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS builder

ARG TARGETOS
ARG TARGETARCH

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -ldflags="-s -w" -o /out/max2tg .

FROM alpine:3.21

RUN apk add --no-cache ca-certificates tini

WORKDIR /app

COPY --from=builder /out/max2tg /app/max2tg

VOLUME ["/app/data"]

ENTRYPOINT ["/sbin/tini", "--", "/app/max2tg"]
