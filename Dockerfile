FROM alpine:3.5

ADD ./cert-exporter /cert-exporter

ENTRYPOINT ["/cert-exporter"]
