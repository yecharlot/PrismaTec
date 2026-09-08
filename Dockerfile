# Build stage — resilient for Render free tier (timeouts / flaky pulls)
FROM golang:1.26-alpine AS builder

RUN apk --no-cache add git ca-certificates

WORKDIR /app

ENV GOPROXY=https://proxy.golang.org,direct \
    GOSUMDB=sum.golang.org \
    CGO_ENABLED=0 \
    GOOS=linux \
    GOFLAGS="-trimpath"

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go build -ldflags="-s -w" -o /app/PrismaTec ./cmd/prisma-tec

# Runtime stage
FROM alpine:3.20

RUN apk --no-cache add ca-certificates tzdata \
    && adduser -D -H -u 1000 appuser

WORKDIR /home/appuser

COPY --from=builder /app/PrismaTec ./PrismaTec
# Apps estáticas (Sales Hub, assets, etc.). Debe ser escribible por appuser
# para que ensurePrismatecApp / ensureFinanzasApp actualicen el HTML embebido.
COPY --from=builder /app/static ./static

RUN mkdir -p /home/appuser/data /home/appuser/alset_data \
    && chown -R appuser:appuser /home/appuser

USER appuser

EXPOSE 8080

CMD ["./PrismaTec"]
