FROM hashicorp/packer:full

RUN apk add --no-cache aws-cli curl

# Pre-provision plugins relevant to AWS
RUN packer plugin install github.com/hashicorp/amazon
RUN packer plugin install github.com/bdwyertech/aws
RUN packer plugin install github.com/bdwyertech/chef