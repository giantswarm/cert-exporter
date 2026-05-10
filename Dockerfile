FROM --platform=$BUILDPLATFORM gsoci.azurecr.io/giantswarm/alpine:3.20.3-giantswarm AS prep
USER root
RUN apk add --no-cache ca-certificates

FROM gsoci.azurecr.io/giantswarm/alpine:3.20.3-giantswarm

COPY --from=prep /etc/ssl/certs /etc/ssl/certs
COPY --from=prep /usr/share/ca-certificates /usr/share/ca-certificates

USER giantswarm

ARG TARGETARCH
COPY ./cert-exporter-linux-${TARGETARCH} /cert-exporter

ENTRYPOINT ["/cert-exporter"]
