FROM golang:1.23-alpine AS build

WORKDIR /src
COPY . .
WORKDIR /src/services/auth-service
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/server ./cmd/server
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/seed-demo ./cmd/seed-demo

FROM alpine:3.20

RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=build /out/server /app/server
COPY --from=build /out/seed-demo /app/seed-demo
EXPOSE 8081
ENTRYPOINT ["/app/server"]
