FROM golang:1.22.4 AS build
WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GOMAXPROCS=1 go build -p 1 -o /bin/iiot-go-api ./main.go

FROM alpine:3.19
WORKDIR /app
RUN apk add --no-cache ca-certificates curl
COPY --from=build /bin/iiot-go-api /usr/local/bin/iiot-go-api
EXPOSE 3001
CMD ["iiot-go-api"]
