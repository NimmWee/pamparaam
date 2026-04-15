FROM golang:1.23-alpine AS build

WORKDIR /src
COPY . .
WORKDIR /src/services/page-service
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/server ./cmd/server

FROM alpine:3.20

RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=build /out/server /app/server
EXPOSE 8082
ENTRYPOINT ["/app/server"]
