FROM quay.io/giantswarm/alpine:3.14.2-giantswarm

RUN apk add --no-cache ca-certificates

COPY ./cert-exporter /cert-exporter

ENTRYPOINT ["/cert-exporter"]
