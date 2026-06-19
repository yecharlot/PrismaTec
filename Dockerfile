# Etapa 1: Compilación
FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY main.go .
RUN go build -o PrismaTec main.go

# Etapa 2: Imagen ligera para producción
FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/PrismaTec .
EXPOSE 8080
CMD ["./PrismaTec"]
