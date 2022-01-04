FROM golang:1.16-stretch AS builder

ARG lfs_version=v3.0.2
WORKDIR /tmp
RUN wget https://github.com/git-lfs/git-lfs/releases/download/${lfs_version}/git-lfs-linux-amd64-${lfs_version}.tar.gz
RUN tar --extract --verbose --file git-lfs-linux-amd64-${lfs_version}.tar.gz

WORKDIR /go/src/github.com/samcontesse/gitlab-merge-request-resource/

COPY . .

RUN GOARCH=amd64 GOOS=linux && \
    go build -o assets/in in/cmd/main.go && \
    go build -o assets/out out/cmd/main.go && \
    go build -o assets/check check/cmd/main.go

FROM concourse/buildroot:git
COPY --from=builder /go/src/github.com/samcontesse/gitlab-merge-request-resource/assets/* /opt/resource/
COPY --from=builder /tmp/git-lfs /usr/local/bin/
