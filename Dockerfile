FROM golang:1.25 AS builder
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 go build -o /datasim ./cmd/datasim

FROM alpine:latest
COPY --from=builder /datasim /usr/local/bin/datasim
ENTRYPOINT ["datasim"]
