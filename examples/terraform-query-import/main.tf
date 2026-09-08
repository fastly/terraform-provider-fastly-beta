terraform {
  required_providers {
    fastly = {
      source  = "fastly/fastly-beta"
    }
  }
}

provider "fastly" {}
