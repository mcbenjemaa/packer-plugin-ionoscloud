# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

# For full specification on the configuration of this file visit:
# https://github.com/hashicorp/integration-template#metadata-configuration
integration {
  name = "IONOS Cloud"
  description = "The IONOS Cloud plugin can be used with HashiCorp Packer to create custom images on IONOS Compute Engine."
  identifier = "packer/hashicorp/ionoscloud"
  component {
    type = "builder"
    name = "IONOSCloud"
    slug = "ionoscloud"
  }
}
