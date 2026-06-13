FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/mockhttpserver/*.go ./
RUN CGO_ENABLED=0 GOOS=linux go build -o mockhttpserver .

FROM alpine:3.19
COPY --from=builder /app/mockhttpserver /app/mockhttpserver
CMD ["/app/mockhttpserver"]
