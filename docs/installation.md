# Installation

### Binary Download

confd ships binaries for OS X, Linux, and Windows for both amd64 and arm64 architectures. You can download the latest release from [GitHub](https://github.com/abtreece/confd/releases).

#### OS X

```bash
# For Intel Macs (amd64)
curl -SL https://github.com/abtreece/confd/releases/download/v0.32.0/confd-v0.32.0-darwin-amd64.tar.gz | tar -xz -C /usr/local/bin/

# For Apple Silicon (arm64)
curl -SL https://github.com/abtreece/confd/releases/download/v0.32.0/confd-v0.32.0-darwin-arm64.tar.gz | tar -xz -C /usr/local/bin/
```

#### Linux

Download and extract the binary:
```bash
# For amd64
curl -SL https://github.com/abtreece/confd/releases/download/v0.32.0/confd-v0.32.0-linux-amd64.tar.gz | tar -xz -C /usr/local/bin/

# For arm64
curl -SL https://github.com/abtreece/confd/releases/download/v0.32.0/confd-v0.32.0-linux-arm64.tar.gz | tar -xz -C /usr/local/bin/
```

Or manually:
```bash
wget https://github.com/abtreece/confd/releases/download/v0.32.0/confd-v0.32.0-linux-amd64.tar.gz
tar -xzf confd-v0.32.0-linux-amd64.tar.gz
mv confd /usr/local/bin/
```

#### Windows

Download the appropriate `.zip` file from the [releases page](https://github.com/abtreece/confd/releases) and extract `confd.exe` to a directory in your PATH.

#### Docker

```dockerfile
ARG CONFD_VERSION=0.32.0
RUN CONFD_ARCH=$(arch | sed s/aarch64/arm64/ | sed s/x86_64/amd64/) \
    && curl -SL "https://github.com/abtreece/confd/releases/download/v${CONFD_VERSION}/confd-v${CONFD_VERSION}-linux-${CONFD_ARCH}.tar.gz" | tar -xz -C /usr/local/bin/ \
    && confd -version
```

#### Building from Source

```bash
make build
make install
```

#### Building from Source for Alpine Linux

Since many people are using Alpine Linux as their base images for Docker there's support to build Alpine package also. Naturally by using Docker itself. :)

```bash
docker build -t confd_builder -f Dockerfile.build.alpine .
docker run -ti --rm -v $(pwd):/app confd_builder make build
```

The above docker commands will produce binary in the local bin directory.

#### Build for your Image using Multi-Stage build

With multi-stage builds you can keep the whole process contained in your Dockerfile using:

```dockerfile
FROM golang:1.23-alpine as confd

ARG CONFD_VERSION=0.32.0

ADD https://github.com/abtreece/confd/archive/v${CONFD_VERSION}.tar.gz /tmp/

RUN apk add --no-cache \
    bzip2 \
    make && \
  mkdir -p /go/src/github.com/abtreece/confd && \
  cd /go/src/github.com/abtreece/confd && \
  tar --strip-components=1 -zxf /tmp/v${CONFD_VERSION}.tar.gz && \
  go install github.com/abtreece/confd/cmd/confd && \
  rm -rf /tmp/v${CONFD_VERSION}.tar.gz

FROM tomcat:8.5-jre8-alpine

COPY --from=confd /go/bin/confd /usr/local/bin/confd

# Then do other useful things...
```

### Next Steps

Get up and running with the [Quick Start Guide](quick-start-guide.md).
