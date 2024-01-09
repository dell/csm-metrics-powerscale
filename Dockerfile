ARG BASEIMAGE

# Build the sdk image
FROM golang:1.21 as builder

# Set envirment variable
ENV APP_NAME csm-metrics-powerscale
ENV CMD_PATH cmd/metrics-powerscale/main.go

# Copy application data into image
COPY . /go/src/$APP_NAME
WORKDIR /go/src/$APP_NAME

# Build
RUN go install github.com/golang/mock/mockgen@v1.6.0
RUN go generate ./...
RUN CGO_ENABLED=0 GOOS=linux go build -o /go/src/service /go/src/$APP_NAME/$CMD_PATH

FROM $BASEIMAGE as final
LABEL vendor="Dell Inc." \
      name="csm-metrics-powerscale" \
      summary="Dell Container Storage Modules (CSM) for Observability - Metrics for PowerScale" \
      description="Provides insight into storage usage and performance as it relates to the CSI (Container Storage Interface) Driver for Dell PowerScale" \
      version="2.0.0" \
      license="Apache-2.0"

COPY --from=builder /go/src/service /
ENTRYPOINT ["/service"]
