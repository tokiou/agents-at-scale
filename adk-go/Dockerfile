FROM golang:1.25-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o /out/server ./cmd/server

FROM alpine:3.22

RUN adduser -D -H appuser
WORKDIR /app

COPY --from=builder /out/server ./server

USER appuser
EXPOSE 8080

ENTRYPOINT ["/app/server"]
