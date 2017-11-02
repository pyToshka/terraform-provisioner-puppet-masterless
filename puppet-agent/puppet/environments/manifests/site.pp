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
