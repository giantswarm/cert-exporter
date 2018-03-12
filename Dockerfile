FROM alpine:3.5

RUN apk add --update ca-certificates \
    && rm -rf /var/cache/apk/*

ADD ./cert-exporter /cert-exporter

ENTRYPOINT ["/cert-exporter"]
