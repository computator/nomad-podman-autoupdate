job "example" {
  group "main" {
    network {
      port "www" {
        to = 8001
      }
    }

    service {
      provider = "nomad"
      port     = "www"
    }

    task "web" {
      driver = "podman"

      meta {
        autoupdate_imgtag_target = ":1"
      }

      config {
        image   = "busybox${NOMAD_META_autoupdate_imgtag_target}"
        command = "httpd"
        args    = ["-v", "-f", "-p", "${NOMAD_PORT_www}", "-h", "/local"]
        ports   = ["www"]
      }

      template {
        data        = <<-EOF
        <h1>Hello, Nomad!</h1>
        <ul>
          <li>Container version: {{env "NOMAD_META_autoupdate_imgtag_target"}}</li>
          <li>Currently running on port: {{env "NOMAD_PORT_www"}}</li>
        </ul>
        EOF
        destination = "local/index.html"
      }
    }
  }
}
