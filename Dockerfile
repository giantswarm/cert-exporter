FROM gsoci.azurecr.io/giantswarm/alpine:3.20.3-giantswarm

USER root

RUN apk add --no-cache ca-certificates

USER giantswarm

ARG TARGETARCH
COPY ./cert-exporter-linux-${TARGETARCH} /cert-exporter

ENTRYPOINT ["/cert-exporter"]
