locals {
  ci    = contains(["1", "true"], var.ci)
  debug = contains(["1", "true"], var.debug)
}
