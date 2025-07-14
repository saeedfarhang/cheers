FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY go.* ./
RUN go mod download

COPY . .

RUN go build -o /go-cheers-app .

FROM alpine:latest

RUN apk add --no-cache ca-certificates

WORKDIR /root/

COPY --from=builder /go-cheers-app .

COPY --from=builder /app/templates ./templates

EXPOSE 8080

CMD ["./go-cheers-app"]