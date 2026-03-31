FROM golang:1.26 AS builder

WORKDIR /src

COPY go.mod ./
COPY cmd ./cmd
COPY internal ./internal

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/telegram-admin-bot ./cmd/bot

FROM gcr.io/distroless/static-debian12

WORKDIR /app

COPY --from=builder /out/telegram-admin-bot /app/telegram-admin-bot

ENTRYPOINT ["/app/telegram-admin-bot"]
