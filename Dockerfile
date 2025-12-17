FROM golang:1.25-alpine AS helper
WORKDIR /go/src/
COPY packer-artifactory-init/ .
RUN GOFLAGS=-mod=vendor CGO_ENABLED=0 go build -ldflags="-s -w" .

FROM hashicorp/packer:light

COPY --from=helper /go/src/packer-artifactory-init /usr/local/bin/
RUN chmod 4755 /usr/local/bin/packer-artifactory-init

RUN apk add --no-cache aws-cli curl

# Pre-provision plugins relevant to AWS
RUN packer plugin install github.com/hashicorp/amazon
RUN packer plugin install github.com/bdwyertech/aws
RUN packer plugin install github.com/bdwyertech/chef
