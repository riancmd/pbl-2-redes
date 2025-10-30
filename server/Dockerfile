# Estágio 1: Build
FROM golang:1.25-alpine AS builder
WORKDIR /src

# Copia go.mod/go.sum da pasta raiz do projeto (o 'contexto')
COPY go.mod go.sum ./
RUN go mod download

# Copia a pasta 'models' da raiz do contexto
COPY models ./models
# Copia o código fonte do servidor (da pasta 'server' do contexto) para uma subpasta 'server'
COPY server/. ./server/

# Muda o diretório de trabalho para a pasta do código do servidor e compila
WORKDIR /src/server
RUN go build -o /app/server .

# Estágio 2: Final - Imagem leve apenas com o executável
FROM alpine:latest
WORKDIR /app

# Adiciona certificados (boa prática)
RUN apk --no-cache add ca-certificates

# Copia o binário compilado do estágio de build
COPY --from=builder /app/server .

# Comando padrão para executar o servidor quando o container iniciar
CMD ["./server"]