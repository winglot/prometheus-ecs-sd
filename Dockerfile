FROM golang:1.22-alpine as builder
RUN apk add --no-cache make git

WORKDIR /go/src/github.com/winglot/prometheus-ecs-sd

COPY go.mod .
COPY go.sum .

RUN go mod download

COPY . .
RUN make build

FROM alpine:3.19
RUN apk add --no-cache ca-certificates

COPY --from=builder /go/src/github.com/winglot/prometheus-ecs-sd/bin/prometheus-ecs-sd /bin/prometheus-ecs-sd
RUN chmod 777 /bin/prometheus-ecs-sd && chown nobody:nobody /bin/prometheus-ecs-sd

USER nobody
ENTRYPOINT ["/bin/prometheus-ecs-sd"]
