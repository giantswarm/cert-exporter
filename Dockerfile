FROM alpine:3.14.1

RUN apk add --update ca-certificates \
    && rm -rf /var/cache/apk/*

ADD ./cert-exporter /cert-exporter

ENTRYPOINT ["/cert-exporter"]
