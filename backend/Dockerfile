FROM golang:1.25-alpine AS build
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
USER estus
EXPOSE 8080
ENTRYPOINT ["./api"]
