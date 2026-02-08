FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /bin/iiot-go-api ./main.go

FROM alpine:3.19
WORKDIR /app
COPY --from=build /bin/iiot-go-api /usr/local/bin/iiot-go-api
EXPOSE 3001
CMD ["iiot-go-api"]
