The ionoscloud plugin allows you to create custom images on the IONOS Compute Engine platform.

## Installation

To install this plugin, copy and paste this code into your Packer configuration, then run [`packer init`](https://www.packer.io/docs/commands/init).

```hcl
packer {
  required_plugins {
    ionoscloud = {
      version = ">= 1.0.0"
      source  = "github.com/hashicorp/ionoscloud"
    }
  }
}
```

Alternatively, you can use `packer plugins install` to manage installation of this plugin.

```sh
$ packer plugins install github.com/hashicorp/ionoscloud
```

## Components

### Builders

- [ionoscloud](/packer/integrations/hashicorp/ionoscloud/latest/components/builder/ionoscloud) - The IONOSCloud Builder
  is able to create virtual machines for [IONOS Compute Engine](https://cloud.ionos.com/compute).
