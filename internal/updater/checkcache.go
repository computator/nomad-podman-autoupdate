package updater

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"

	"go.podman.io/podman/v6/pkg/inspect"

	"nomad-podman-autoupdate/internal/podmanutil"
)

type CheckCache struct {
	PodmanConn context.Context
	imageData  map[string]*inspect.ImageData
	imgIds     map[string]string
	pullQueue  map[string]<-chan error
	queueMutex sync.Mutex
}

func NewCheckCache(podmanConn context.Context) *CheckCache {
	return &CheckCache{
		PodmanConn: podmanConn,
		imageData:  make(map[string]*inspect.ImageData),
		imgIds:     make(map[string]string),
		pullQueue:  make(map[string]<-chan error),
	}
}

func (c *CheckCache) Check(imgRef string) (*inspect.ImageData, error) {
	checkLogger := slog.With(slog.String("image", imgRef))

	if id, ok := c.imgIds[imgRef]; ok {
		checkLogger.Debug("returning previously cached image", slog.String("image_id", id))
		return c.imageData[id], nil
	}

	c.queueMutex.Lock()
	queueChan, wasQueued := c.pullQueue[imgRef]
	if !wasQueued {
		newChan := make(chan error)
		c.pullQueue[imgRef] = newChan
		c.queueMutex.Unlock()

		checkLogger.Info("queuing check for image")
		go c.fetchImageToCache(newChan, imgRef)
		queueChan = newChan
	} else {
		c.queueMutex.Unlock()
	}

	checkLogger.Debug("waiting for queued image check")
	select {
	case err := <-queueChan:
		if err != nil {
			checkLogger.Warn("queued image check failed with error", slog.Any("err", err))
			return nil, fmt.Errorf("error checking image in queue: %w", err)
		}
	}

	if id, ok := c.imgIds[imgRef]; ok {
		checkLogger.Debug("returning newly cached image", slog.String("image_id", id))
		return c.imageData[id], nil
	} else {
		return nil, errors.New("previous queued image check failed")
	}
}

func (c *CheckCache) fetchImageToCache(queueChan chan<- error, imgRef string) {
	defer close(queueChan)

	_, err := podmanutil.PullImage(c.PodmanConn, imgRef)
	if err != nil {
		queueChan <- fmt.Errorf("failed to pull image: %w", err)
		return
	}
	imgInfo, err := podmanutil.ImageInfo(c.PodmanConn, imgRef)
	if err != nil {
		queueChan <- fmt.Errorf("failed to inspect image: %w", err)
		return
	}

	c.imageData[imgInfo.ID] = imgInfo
	c.imgIds[imgRef] = imgInfo.ID
}
