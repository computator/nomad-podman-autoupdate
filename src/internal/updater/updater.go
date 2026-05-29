package updater

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"

	nomadApi "github.com/hashicorp/nomad/api"

	"nomad-podman-autoupdate/internal/common"
	"nomad-podman-autoupdate/internal/nomadutil"
)

type Updater struct {
	NomadClient *nomadApi.Client
	PodmanConn  context.Context
	ccache      *CheckCache
}

func NewUpdater(nomadClient *nomadApi.Client, podmanConn context.Context) *Updater {
	return &Updater{
		NomadClient: nomadClient,
		PodmanConn:  podmanConn,
		ccache:      NewCheckCache(podmanConn),
	}
}

func (u *Updater) TryUpdateJob(jobId string) error {
	job, err := nomadutil.GetJobInfo(u.NomadClient, jobId)
	if err != nil {
		return fmt.Errorf("failed to load job '%s': %w", jobId, err)
	}

	var (
		taskFound    = false
		taskUpdated  = false
		updateErrors = false
		wg           sync.WaitGroup
	)
	for _, g := range job.TaskGroups {
		for _, t := range g.Tasks {
			if oldTarget, ok := t.Meta[common.UpdateableTaskMetaTarget]; ok {
				taskFound = true

				taskLogger := slog.With(
					slog.String("job", jobId),
					slog.String("group", *g.Name),
					slog.String("task", t.Name),
				)

				wg.Go(func() {
					success, err := (&taskUpdater{
						Updater: u,
						logger:  taskLogger,
						job:     job,
						task:    t,
					}).tryUpdateTask()
					if err != nil {
						taskLogger.Warn("failed to update task", slog.Any("err", err))
						updateErrors = true
						return
					}

					if success {
						taskUpdated = true

						newTarget, ok := t.Meta[common.UpdateableTaskMetaTarget]
						if !ok {
							// use slog since this indicates a bug and isn't task specific
							slog.Error("expected meta property '" + common.UpdateableTaskMetaTarget + "' has been unset")
							updateErrors = true
							return
						}

						taskLogger.Info("task update found",
							slog.String("target", newTarget),
							slog.String("old_target", oldTarget),
						)
					} else {
						taskLogger.Debug("no update found for task")
					}
				})
			}
		}
	}
	if !taskFound {
		return fmt.Errorf("no updatable tasks found in job '%s'", jobId)
	}

	wg.Wait()

	if updateErrors {
		return fmt.Errorf("one or more errors encountered while updating job '%s'", jobId)
	}

	if !taskUpdated {
		slog.Info("no task updates found for job", slog.String("job", jobId))
		return nil
	}

	oldIndex := new(int(*job.JobModifyIndex))
	newIndex, err := nomadutil.UpsertJob(u.NomadClient, job, oldIndex)
	if err != nil {
		if errors.Is(err, nomadutil.ErrModifyIndexConflict) {
			return fmt.Errorf("job modified elsewhere during update process '%s': %w", jobId, err)
		} else {
			return fmt.Errorf("failed to submit modified job with updated tasks '%s': %w", jobId, err)
		}
	}
	slog.Info("submitted modified job with updated tasks", slog.String("job", jobId), slog.Int("job_index", newIndex))

	return nil
}

type taskUpdater struct {
	logger *slog.Logger
	job    *nomadApi.Job
	task   *nomadApi.Task
	*Updater
}

func (tu *taskUpdater) tryUpdateTask() (bool, error) {
	imgPullRef, err := tu.getImagePullRef()
	if err != nil {
		return false, fmt.Errorf("failed to get current task image reference: %w", err)
	}

	imgInfo, err := tu.ccache.Check(imgPullRef)
	if err != nil {
		return false, fmt.Errorf("failed to check for new image: %w", err)
	}

	currDigestTag, err := tu.getRefDigest()
	if err != nil {
		return false, fmt.Errorf("failed to get current task image digest reference: %w", err)
	}

	newRefTag := "@" + imgInfo.Digest.String()
	if newRefTag == currDigestTag {
		tu.logger.Debug(
			"pulled image digest matches current task digest reference",
			slog.String("image", imgPullRef),
			slog.String("digest", imgInfo.Digest.String()),
		)
		return false, nil
	}

	tu.logger.Debug(
		"new digest found for task image",
		slog.String("image", imgPullRef),
		slog.String("digest", imgInfo.Digest.String()),
	)

	if err := tu.setImageRefData(newRefTag); err != nil {
		return false, fmt.Errorf("failed to set new task image reference data: %w", err)
	}

	return true, nil
}
