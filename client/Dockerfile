# Estágio 1: Build
FROM golang:1.25-alpine AS builder
WORKDIR /src

# Copia go.mod/go.sum da pasta raiz do projeto (o 'contexto')
COPY go.mod go.sum ./
RUN go mod download

# Copia a pasta 'models' da raiz do contexto
COPY models ./models
# Copia o código fonte do cliente (da pasta 'client' do contexto) para uma subpasta 'client'
COPY client/. ./client/

# Muda o diretório de trabalho para a pasta do código do cliente e compila
WORKDIR /src/client
RUN go build -o /app/client .

# Estágio 2: Final - Imagem leve apenas com o executável
FROM alpine:latest
WORKDIR /app

# Adiciona certificados (boa prática)
RUN apk --no-cache add ca-certificates

# Copia o binário compilado do estágio de build
COPY --from=builder /app/client .

# Comando padrão para executar o cliente quando o container iniciar
CMD ["./client"]