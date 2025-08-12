# --- Estágio 1: Build ---
    FROM golang:1.24-alpine AS builder

    WORKDIR /app
    
    COPY go.mod go.sum ./
    RUN go mod download
    
    COPY . .
    
    # Compilação simplificada. O Go usará a arquitetura correta por padrão.
    RUN CGO_ENABLED=0 GOOS=linux go build -o /app/main .
    
    # --- Estágio 2: Final ---
    FROM alpine:latest
    
    WORKDIR /app
    
    COPY --from=builder /app/main .
    
    # A permissão de execução ainda é necessária
    RUN chmod +x /app/main
    
    EXPOSE 8080
    
    CMD ["/app/main"]