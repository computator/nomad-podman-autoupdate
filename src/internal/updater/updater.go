package updater

import (
	"fmt"
	"log/slog"

	nomadApi "github.com/hashicorp/nomad/api"
)

type Updater struct {
	nomad *nomadApi.Client
}

func NewUpdater(nomad *nomadApi.Client) *Updater {
	return &Updater{
		nomad: nomad,
	}
}

func (u *Updater) GetUpdateableJobs() ([]string, error) {
	jobs, _, err := u.nomad.Jobs().List(&nomadApi.QueryOptions{})
	if err != nil {
		return []string{}, fmt.Errorf("failed to list nomad jobs: %w", err)
	}
	slog.Debug("loaded nomad jobs", slog.Int("count", len(jobs)))
	jobIds := make([]string, 0, len(jobs))
	for _, j := range jobs {
		slog.Debug("got job listing", slog.Any("job", j))
		jobIds = append(jobIds, j.ID)
	}
	slog.Debug("job id list", slog.Any("ids", jobIds))
	return jobIds, nil
}

func (u *Updater) GetJobSpec(jobId string) (*nomadApi.JobSubmission, error) {
	job, _, err := u.nomad.Jobs().Info(jobId, &nomadApi.QueryOptions{})
	if err != nil {
		return &nomadApi.JobSubmission{}, fmt.Errorf("failed to get info for nomad job '%s': %w", jobId, err)
	}
	slog.Debug("got job info", slog.String("id", jobId), slog.Any("job", job))

	jobSrc, _, err := u.nomad.Jobs().Submission(jobId, int(*job.Version), &nomadApi.QueryOptions{})
	if err != nil {
		return &nomadApi.JobSubmission{}, fmt.Errorf("failed to get source for nomad job '%s': %w", jobId, err)
	}
	slog.Debug("got job source", slog.String("id", jobId), slog.Any("source", jobSrc))

	return jobSrc, nil
}
