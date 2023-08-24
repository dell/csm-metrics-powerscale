FROM  registry.access.redhat.com/ubi9/ubi-micro@sha256:630cf7bdef807f048cadfe7180d6c27eb3aaa99323ffc3628811da230ed3322a
LABEL vendor="Dell Inc." \
      name="csm-metrics-powerscale" \
      summary="Dell Container Storage Modules (CSM) for Observability - Metrics for PowerScale" \
      description="Provides insight into storage usage and performance as it relates to the CSI (Container Storage Interface) Driver for Dell PowerScale" \
      version="2.0.0" \
      license="Apache-2.0"
ARG SERVICE
COPY $SERVICE/bin/service /service
ENTRYPOINT ["/service"]
