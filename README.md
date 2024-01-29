<!--
Copyright (c) 2022 Dell Inc., or its subsidiaries. All Rights Reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0
-->

# Observability Module for PowerScale

[![Contributor Covenant](https://img.shields.io/badge/Contributor%20Covenant-v2.0%20adopted-ff69b4.svg)](https://github.com/dell/csm/blob/main/docs/CODE_OF_CONDUCT.md)
[![License](https://img.shields.io/github/license/dell/csm-metrics-powerscale)](LICENSE)
[![Docker Pulls](https://img.shields.io/docker/pulls/dellemc/csm-metrics-powerscale)](https://hub.docker.com/r/dellemc/csm-metrics-powerscale)
[![Go version](https://img.shields.io/github/go-mod/go-version/dell/csm-metrics-powerscale)](go.mod)
[![GitHub release (latest by date including pre-releases)](https://img.shields.io/github/v/release/dell/csm-metrics-powerscale?include_prereleases&label=latest&style=flat-square)](https://github.com/dell/csm-metrics-powerscale/releases/latest)

Metrics for PowerScale is part of Dell Container Storage Modules (CSM) for Observability, which provides Kubernetes administrators standardized approaches for storage observability in Kuberenetes environments.

Metrics for PowerScale is an open source distributed solution that provides insight into storage usage and performance as it relates to the CSI (Container Storage Interface) Driver for Dell PowerScale.

Metrics for PowerScale captures telemetry data of storage usage and performance obtained through the CSI Driver for Dell PowerScale. The Metrics service pushes it to the OpenTelemetry Collector, so it can be processed, and exported in a format consumable by Prometheus. Prometheus can then be configured to scrape the OpenTelemetry Collector exporter endpoint to provide metrics so they can be visualized in Grafana.

For documentation, please visit [Container Storage Modules documentation](https://dell.github.io/csm-docs/).

## Table of Contents

- [Code of Conduct](https://github.com/dell/csm/blob/main/docs/CODE_OF_CONDUCT.md)
- [Maintainer Guide](https://github.com/dell/csm/blob/main/docs/MAINTAINER_GUIDE.md)
- [Committer Guide](https://github.com/dell/csm/blob/main/docs/COMMITTER_GUIDE.md)
- [Contributing Guide](https://github.com/dell/csm/blob/main/docs/CONTRIBUTING.md)
- [List of Adopters](https://github.com/dell/csm/blob/main/docs/ADOPTERS.md)
- [Support](https://github.com/dell/csm/blob/main/docs/SUPPORT.md)
- [Security](https://github.com/dell/csm/blob/main/docs/SECURITY.md)
- [About](#about)

## Building Metrics for PowerScale

If you wish to clone and build the Metrics for PowerScale service, a Linux host is required with the following installed:

| Component       | Version   | Additional Information                                                                                                                     |
| --------------- | --------- | ------------------------------------------------------------------------------------------------------------------------------------------ |
| Docker          | v19+      | [Docker installation](https://docs.docker.com/engine/install/)                                                                                                    |
| Docker Registry |           | Access to a local/corporate [Docker registry](https://docs.docker.com/registry/)                                                           |
| Golang          | v1.18+    | [Golang installation](https://github.com/travis-ci/gimme)                                                                                                         |
| gosec           |           | [gosec](https://github.com/securego/gosec)                                                                                                          |
| gomock          | v.1.6.0   | [Go Mock](https://github.com/golang/mock)                                                                                                             |
| git             | latest    | [Git installation](https://git-scm.com/book/en/v2/Getting-Started-Installing-Git)                                                                              |
| gcc             |           | Run ```sudo apt install build-essential```                                                                                                 |
| kubectl         | 1.23.5    | Ensure you copy the kubeconfig file from the Kubernetes cluster to the linux host. [kubectl installation](https://kubernetes.io/docs/tasks/tools/install-kubectl/) |
| Helm            | v.3.8.1   | [Helm installation](https://helm.sh/docs/intro/install/)                                                                                                        |

Once all prerequisites are on the Linux host, follow the steps below to clone and build the metrics service:

1. Clone the repository using the following command: `git clone https://github.com/dell/csm-metrics-powerscale.git`
2. Set the DOCKER_REPO environment variable to point to the local Docker repository, for example: `export DOCKER_REPO=<ip-address>:<port>`
3. In the csm-metrics-powerscale directory, run the following command to build the Docker image called csm-metrics-powerscale: `make docker`
4. Tag (with the "latest" tag) and push the image to the local Docker repository by running the following command: `make tag push`

__Note:__ Linux support only. If you are using a local insecure docker registry, ensure you configure the insecure registries on each of the Kubernetes worker nodes to allow access to the local docker repository.

## Testing the Observability Module for PowerScale

From the root directory where the repo was cloned, the unit tests can be executed using the following command:

```console
make test
```

This will also provide code coverage statistics for the various Go packages.

## Versioning

This project is adhering to [Semantic Versioning](https://semver.org/).

## About

Dell Container Storage Modules (CSM) is 100% open source and community-driven. All components are available
under [Apache 2 License](https://www.apache.org/licenses/LICENSE-2.0.html) on
GitHub.
