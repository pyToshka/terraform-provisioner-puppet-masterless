output "private_ip" {
  value = "${aws_instance.puppetagent.public_ip}"
}

