# Construído a partir da raiz do repositório: docker build -f backend/Dockerfile .
FROM golang:1.26-alpine AS build
WORKDIR /src
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ .
RUN CGO_ENABLED=0 go build -o /bin/api ./cmd/api

FROM alpine:3.20
RUN adduser -D -u 10001 estus
WORKDIR /app
COPY --from=build /bin/api ./api
COPY --from=build /src/migrations ./migrations
# Documentos e o token do MCP ficam em /app/data: monte um volume aí.
RUN mkdir -p data && chown estus data
USER estus
EXPOSE 8080
ENTRYPOINT ["./api"]
