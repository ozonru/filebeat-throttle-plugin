ARG filebeatVersion=6.5.4
ARG goVersion=1.10.6
FROM golang:$goVersion
ARG filebeatVersion
RUN curl -L --output /tmp/filebeat.tar.gz https://github.com/elastic/beats/archive/v$filebeatVersion.tar.gz

RUN mkdir -p /go/src/github.com/elastic/beats && tar -xvzf /tmp/filebeat.tar.gz --strip-components=1 -C /go/src/github.com/elastic/beats
RUN go get -v golang.org/x/vgo

COPY . /go/src/github.com/ozonru/filebeat-throttle-plugin
COPY register/plugin/plugin.go /go/src/github.com/elastic/beats/libbeat/processors/throttle/plugin.go

RUN (cd /go/src/github.com/ozonru/filebeat-throttle-plugin && vgo mod vendor -v)
RUN rm -rf /go/src/github.com/ozonru/filebeat-throttle-plugin/vendor/github.com/elastic
RUN go build -v -o /output/filebeat_throttle_linux.so -buildmode=plugin github.com/elastic/beats/libbeat/processors/throttle


FROM docker.elastic.co/beats/filebeat:$filebeatVersion
COPY  --from=0 /output/filebeat_throttle_linux.so /filebeat_throttle_linux.so

CMD ["-e", "--plugin", "/filebeat_throttle_linux.so"]
