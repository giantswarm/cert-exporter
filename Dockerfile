FROM quay.io/giantswarm/alpine:3.18.4-giantswarm

USER root

RUN apk add --no-cache ca-certificates

USER giantswarm

COPY ./cert-exporter /cert-exporter

ENTRYPOINT ["/cert-exporter"]
