# terraform-provisioner-puppet-masterless



> Provision terraform resources with puppet-masterless
> This module create just for fun :)
>

## Overview

**[Terraform](https://github.com/hashicorp/terraform)** is a tool for automating infrastructure. Terraform includes the ability to provision resources at creation time through a plugin api. Currently, some builtin [provisioners](https://www.terraform.io/docs/provisioners/) such as **chef** and standard scripts are provided; this provisioner introduces the ability to provision an instance at creation time with **puppet-masterless**.

This provisioner provides the ability to install puppet agent and try to configure instance.

**terraform-provisioner-puppet-masterless** is shipped as a **Terraform** [module](https://www.terraform.io/docs/modules/create.html). To include it, simply download the binary and enable it as a terraform module in your **terraformrc**.

## Installation
```bash
go get github.com/pyToshka/terraform-provisioner-puppet-masterless

```
**terraform-provisioner-puppet-masterless** ships as a single binary and is compatible with **terraform**'s plugin interface. Behind the scenes, terraform plugins use https://github.com/hashicorp/go-plugin and communicate with the parent terraform process via RPC.

Once installed, a `~/.terraformrc` file is used to _enable_ the plugin.

```bash
providers {
    puppet-masterless = "/usr/local/bin/terraform-puppet-masterless"
}
```

## Usage

Once installed, you can provision resources by including an `puppet-masterless` provisioner block.

The following example demonstrates a configuration block to install and running puppet agent to new instances and simple provisioning.


```
provider "aws" {
  access_key = "${var.access_key}"
  secret_key = "${var.secret_key}"
  region = "${var.region}"

}
resource "aws_instance" "puppetagent" {
  ami = "${var.ami_id}"
  instance_type = "${var.instance_type}"
  count = 1
  tags {
    Name = "test-instance.local"
  }
  root_block_device {
    volume_size = 8
    delete_on_termination = true
  }
  key_name = "${var.aws_key_name}"
  subnet_id = "${var.subnet_id}"
  security_groups = [
    "${var.security_group}"]
  vpc_security_group_ids = [
    "${var.security_group}"]
  provisioner "puppet-masterless" {
    connection {
      user = "ubuntu"
      private_key = "${file("id_rsa")}"
    }
    manifest_file = "puppet/environments/manifests/site.pp"
    prevent_sudo = true
    manifest_dir = "puppet/environments"
    module_paths = "puppet/modules"
  }
}

```

Provisioner options
```
module_paths - This is an array of paths to module directories on your local filesystem. These will be uploaded to the remote machine. By default, this is empty.

manifest_file - This is either a path to a puppet manifest (.pp file) or a directory containing multiple manifests that puppet will apply  this manifest must exist on your local system and will be uploaded to the remote machine.

manifest_dir - The path to a local directory with manifests to be uploaded to the remote machine. This is useful if your main manifest file uses imports.

prevent_sudo - By default, the configured commands that are executed to run Puppet are executed with sudo

staging_directory - By default this is /tmp/terraform-puppet-masterless.

```

example
```bash
git https://github.com/pyToshka/terraform-provisioner-puppet-masterless
cd terraform-provisioner-puppet-masterless/puppet
mv variables.tf_example variables.tf
do some changes and setup your vars
terraform plan
terrafrom apply

```


Sample site.pp from my example below
```ruby
exec { "apt-get update":
  command => "/usr/bin/apt-get update",
}

package { "git":
  require => Exec["apt-get update"],
}
package { "docker":
  require => Exec["apt-get update"],
}
package { "docker.io":
  require => Exec["apt-get update"],
}


```

Possible structure for Puppet environment

```
puppet
|-- environments
|   `-- manifests
|       `-- site.pp
`-- modules
    |-- apt
    |-- puppi
    |-- stdlib

```




I'm not testing this provisioner only on Ubuntu 14.04.
===================================================================================================================

Full working are granted only with Ubuntu based distribution
===================================================================================================================
