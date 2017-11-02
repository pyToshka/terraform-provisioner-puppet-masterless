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
      private_key = "${file("/Users/iuriimedvedev/.ssh/id_rsa")}"
    }
    manifest_file = "puppet/environments/manifests/site.pp"
    prevent_sudo = true
    manifest_dir = "puppet/environments"
    module_paths = "puppet/modules"
  }
}
