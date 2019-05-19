FROM quay.io/giantswarm/alpine:3.9-giantswarm

USER root

RUN apk add --update ca-certificates \
    && rm -rf /var/cache/apk/*

ADD ./cert-exporter /cert-exporter

USER giantswarm

ENTRYPOINT ["/cert-exporter"]
