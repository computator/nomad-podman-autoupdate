job "nomad-podman-autoupdate" {
  type = "batch"

  periodic {
    crons            = ["@daily"]
    prohibit_overlap = true
  }

  group "main" {
    task "nomad-podman-autoupdate" {
      driver = "podman"

      config {
        force_pull   = true
        image        = "ghcr.io/computator/nomad-podman-autoupdate"
        command      = "nomad-podman-autoupdate"
        args         = ["-v"]
        network_mode = "none"
        security_opt = ["label=disable"]
        volumes = [
          "/run/podman/podman.sock:/run/podman/podman.sock"
        ]
      }

      env {
        NOMAD_ADDR = "${NOMAD_UNIX_ADDR}"
      }

      identity {
        env = true
      }
    }
  }
}
