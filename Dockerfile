FROM golang:1.25.3 AS builder

WORKDIR /src

COPY go.mod ./
COPY go.sum ./
RUN go mod download

COPY cmd ./cmd
COPY internal ./internal

RUN CGO_ENABLED=0 go build -trimpath -o /out/telegram-admin-bot ./cmd/bot

FROM gcr.io/distroless/base-debian12

WORKDIR /app

COPY --from=builder /out/telegram-admin-bot /app/telegram-admin-bot

ENTRYPOINT ["/app/telegram-admin-bot"]
