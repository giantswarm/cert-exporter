FROM gsoci.azurecr.io/giantswarm/alpine:3.20.3-giantswarm

USER root

RUN apk add --no-cache ca-certificates

USER giantswarm

COPY ./cert-exporter /cert-exporter

ENTRYPOINT ["/cert-exporter"]
