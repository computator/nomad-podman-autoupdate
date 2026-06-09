job "example-multitask" {
  group "main" {
    network {
      dynamic "port" {
        for_each = range(5)
        labels   = ["www${port.value}"]
        content {
          to = 8081
        }
      }
    }

    dynamic "task" {
      for_each = range(5)
      labels   = ["web${task.value}"]
      content {
        driver = "podman"

        meta {
          autoupdate_imgtag_target = ":1.${38-task.value}"
        }

        config {
          image   = "busybox${NOMAD_META_autoupdate_imgtag_target}"
          command = "httpd"
          args    = ["-v", "-f", "-p", "8081", "-h", "/local"]
          ports   = ["www${task.value}"]
        }

        template {
          data        = <<-EOF
          <h1>Hello, Nomad!</h1>
          <ul>
            <li>Container version: {{env "NOMAD_META_autoupdate_imgtag_target"}}</li>
            <li>Currently running on port: {{env "NOMAD_PORT_www${task.value}"}}</li>
          </ul>
          EOF
          destination = "local/index.html"
        }
      }
    }
  }
}
