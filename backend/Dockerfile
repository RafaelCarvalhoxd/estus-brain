FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
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
