FROM golang:1.22-alpine as builder

WORKDIR /go/src/github.com/winglot/prometheus-ecs-sd

COPY go.mod .
COPY go.sum .

RUN go mod download

COPY . .
RUN go build -a -installsuffix cgo -o /prometheus-ecs-sd ./cmd/sd/main.go

FROM alpine:3.19
RUN apk add -U ca-certificates

COPY --from=builder /prometheus-ecs-sd /prometheus-ecs-sd
RUN chmod 777 /prometheus-ecs-sd && chown nobody:nobody /prometheus-ecs-sd

USER nobody
ENTRYPOINT ["/prometheus-ecs-sd"]
