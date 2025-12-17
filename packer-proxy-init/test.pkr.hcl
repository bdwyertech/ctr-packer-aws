packer {
  required_plugins {
    amazon = {
      source  = "github.com/hashicorp/amazon"
      version = ">= 1.8.0"
    }

    aws = {
      version = "0.0.4"
      source  = "github.com/bdwyertech/aws"
    }
  }
}
