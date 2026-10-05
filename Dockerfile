FROM golang:1.26-alpine AS Builder

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags '-s -w' -o /out/server .


FROM alpine:latest

WORKDIR /app

COPY --from=builder /out/server .

EXPOSE 8080

CMD ["./server"]


