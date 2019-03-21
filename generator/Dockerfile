FROM golang:1.11-stretch

COPY main.go /go/src/gitlab.ozon.ru/sre/logs-generator/main.go

RUN go build -o /usr/local/bin/logs-generator /go/src/gitlab.ozon.ru/sre/logs-generator/main.go

FROM debian:stretch-slim

COPY --from=0 /usr/local/bin/logs-generator /usr/local/bin/logs-generator

ENTRYPOINT ["/usr/local/bin/logs-generator"]
CMD ["-id", "some_id"]
