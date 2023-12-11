Type: `ionoscloud`
Artifact BuilderId: `packer.ionoscloud`

The IONOSCloud Builder is able to create virtual machines for
[IONOS Compute Engine](https://cloud.ionos.com/compute).

## Configuration Reference

There are many configuration options available for the builder. They are
segmented below into two categories: required and optional parameters. Within
each category, the available configuration keys are alphabetized.

In addition to the options listed here, a
[communicator](/docs/templates/legacy_json_templates/communicator) can be configured for this
builder. In addition to the options defined there, a private key file
can also be supplied to override the typical auto-generated key:

- `ssh_private_key_file` (string) - Path to a PEM encoded private key file to use to authenticate with SSH.
  The `~` can be used in path and will be expanded to the home directory
  of current user.


### Required

- `image` (string) - IONOSCloud volume image. Only Linux public images are
  supported. To obtain full list of available images you can use
  [ionos CLI](https://github.com/ionos-cloud/ionosctl/blob/master/docs/subcommands/Compute%20Engine/image/list.md#imagelist).

- `password` (string) - IONOS password. This can be specified via
  environment variable `IONOS_PASSWORD`, if provided. The value
  defined in the config has precedence over environemnt variable.

- `username` (string) - IONOS username. This can be specified via
  environment variable `IONOS_USERNAME`, if provided. The value
  defined in the config has precedence over environemnt variable.

- `ssh_username` (string) - SSH username to use to connect to the instance, *must use `root`*.

- `ssh_private_key_file` (string) - Path to the SSH private key file to use to connect to the instance, *required for ssh*.

### Optional

- `cores` (number) - Amount of CPU cores to use for this build. Defaults to
  "4".

- `disk_size` (string) - Amount of disk space for this image in GB. Defaults
  to "50"

- `disk_type` (string) - Type of disk to use for this image. Defaults to
  "HDD".

- `location` (string) - Defaults to "us/las".

- `ram` (number) - Amount of RAM to use for this image. Defaults to "2048".

- `retries` (string) - Number of retries Packer will make status requests
  while waiting for the build to complete. Default value 120 seconds.

- `snapshot_name` (string) - If snapshot name is not provided Packer will
  generate it

- `snapshot_password` (string) - Password for the snapshot.

- `ssh_timeout` (string) - SSH timeout. Defaults to "10m".

<!-- markdown-link-check-disable -->
- `url` (string) - Endpoint for the IONOS Cloud REST API. Default URL
"<https://api.ionos.com>"
<!-- markdown-link-check-enable -->

## Example

Here is a basic example:

**HCL**

```hcl
source "ionoscloud" "ubuntu" {
  image                 = "Ubuntu-22.04"
  disk_size             = 5
  snapshot_name         = "test-snapshot"
  ssh_username          = "root"
  ssh_password          = "test1234"
  ssh_timeout          = "15m"
  ssh_private_key_file  = "~/.ssh/id_rsa"
}

build {
  sources = "ionoscloud.ubuntu"
}
```

**JSON**

```json
{
  "builders": [
    {
      "communicator": "ssh",
      "name": "ubuntu",
      "snapshot_name": "ubuntu-k8s",
      "image": "Ubuntu-22.04",
      "location": "de/txl",
      "disk_size": "5",
      "ssh_username": "root",
      "ssh_password": "builder0",
      "ssh_timeout": "5m",
      "ssh_private_key_file": "~/.ssh/id_ed25519",
      "type": "ionoscloud"
    }
  ]
}
```
