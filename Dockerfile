# Build stage
FROM golang:1.26-alpine AS builder
RUN apk --no-cache add git ca-certificates
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o PrismaTec ./cmd/prisma-tec

# Runtime stage
FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/PrismaTec .
# Apps y estáticos embebidos en la imagen (sobreviven al redeploy)
COPY --from=builder /app/static ./static
EXPOSE 8080
CMD ["./PrismaTec"]
