package updater

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"

	"github.com/jackc/puddle/v2"
	"go.podman.io/podman/v6/pkg/inspect"

	"nomad-podman-autoupdate/internal/podmanutil"
)

type CheckCache struct {
	PodmanConnPool *puddle.Pool[context.Context]
	imageData      map[string]*inspect.ImageData
	imgIds         map[string]string
	pullQueue      map[string]<-chan error
	queueMutex     sync.Mutex
}

func NewCheckCache(podmanConnPool *puddle.Pool[context.Context]) *CheckCache {
	return &CheckCache{
		PodmanConnPool: podmanConnPool,
		imageData:      make(map[string]*inspect.ImageData),
		imgIds:         make(map[string]string),
		pullQueue:      make(map[string]<-chan error),
	}
}

func (c *CheckCache) Check(imgRef string) (*inspect.ImageData, error) {
	checkLogger := slog.With(slog.String("image", imgRef))

	if id, ok := c.imgIds[imgRef]; ok {
		checkLogger.Debug("returning previously cached image tag", slog.String("image_id", id))
		return c.imageData[id], nil
	}

	c.queueMutex.Lock()
	queueChan, wasQueued := c.pullQueue[imgRef]
	if !wasQueued {
		newChan := make(chan error)
		c.pullQueue[imgRef] = newChan
		c.queueMutex.Unlock()

		checkLogger.Debug("initializing image check for queue")
		go c.fetchImageToCache(newChan, imgRef)
		queueChan = newChan
	} else {
		c.queueMutex.Unlock()
	}

	checkLogger.Debug("waiting for queued image check")
	select {
	case err := <-queueChan:
		checkLogger.Debug("got result from queued image check")
		if err != nil {
			checkLogger.Warn("queued image check failed with error", slog.Any("err", err))
			return nil, fmt.Errorf("error checking image in queue: %w", err)
		}
	}

	if id, ok := c.imgIds[imgRef]; ok {
		checkLogger.Debug("returning cached image tag from queued check", slog.String("image_id", id))
		return c.imageData[id], nil
	} else {
		return nil, errors.New("previous queued image check failed")
	}
}

func (c *CheckCache) fetchImageToCache(queueChan chan<- error, imgRef string) {
	defer close(queueChan)

	connRsrc, err := c.PodmanConnPool.Acquire(context.Background())
	if err != nil {
		queueChan <- fmt.Errorf("failed to acquire podman connection: %w", err)
		return
	}
	defer connRsrc.Release()

	slog.Info("fetching image for tag", slog.String("image", imgRef))

	_, err = podmanutil.PullImage(connRsrc.Value(), imgRef)
	if err != nil {
		queueChan <- fmt.Errorf("failed to pull image: %w", err)
		return
	}
	imgInfo, err := podmanutil.ImageInfo(connRsrc.Value(), imgRef)
	if err != nil {
		queueChan <- fmt.Errorf("failed to inspect image: %w", err)
		return
	}

	c.imageData[imgInfo.ID] = imgInfo
	c.imgIds[imgRef] = imgInfo.ID
}
