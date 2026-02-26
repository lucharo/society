FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /society ./cmd/society

FROM alpine:3.20
COPY --from=builder /society /usr/local/bin/society
COPY agents/ /etc/society/agents/
ENTRYPOINT ["society"]
