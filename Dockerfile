ARG filebeatVersion=6.6.2

FROM docker.elastic.co/beats/filebeat:$filebeatVersion

COPY filebeat_throttle_linux.so /filebeat_throttle_linux.so

CMD ["-e", "--plugin", "/filebeat_throttle_linux.so"]
