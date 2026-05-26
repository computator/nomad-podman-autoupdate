package podmanutil

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"go.podman.io/podman/v6/pkg/bindings"
	"go.podman.io/podman/v6/pkg/bindings/images"
	"go.podman.io/podman/v6/pkg/inspect"
)

const PodmanDefaultURI = "unix:///run/podman/podman.sock"

func NewDefaultConnection() (context.Context, error) {
	var conn context.Context
	uri := os.Getenv("CONTAINER_HOST")
	if uri == "" {
		uri = os.Getenv("DOCKER_HOST")
		if uri == "" {
			uri = PodmanDefaultURI
		}
	}
	conn, err := bindings.NewConnection(context.Background(), uri)
	return conn, err
}

func PullImage(pconn context.Context, ref string) (string, error) {
	images, err := images.Pull(pconn, ref, (&images.PullOptions{}).WithQuiet(true).WithPolicy("newer"))
	if err != nil {
		return "", fmt.Errorf("error pulling container image '%s': %w", ref, err)
	}
	if len(images) != 1 {
		return "", fmt.Errorf("unexpected number of image ids returned %d != 1", len(images))
	}
	slog.Debug("pulled container image", slog.String("ref", ref), slog.Any("id", images[0]))
	return images[0], nil
}

func ImageInfo(pconn context.Context, ref string) (*inspect.ImageData, error) {
	info, err := images.GetImage(pconn, ref, &images.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("error retriving image info '%s': %w", ref, err)
	}
	slog.Debug("inspected container image", slog.String("ref", ref), slog.Any("info", info.ImageData))
	return info.ImageData, nil
}
