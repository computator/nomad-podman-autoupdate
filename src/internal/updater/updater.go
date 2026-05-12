package updater

import (
	"fmt"
	"log/slog"

	nomadApi "github.com/hashicorp/nomad/api"
)

const (
	UpdateableTaskMetaTarget = "autoupdate_imgtag_target"
	UpdateableTaskMetaSource = "autoupdate_imgtag_source"
	UpdateableJobsFilterExpr = "any TaskGroups as tg { any tg.Tasks as t" +
		" { " + UpdateableTaskMetaTarget + " in t.Meta } }"
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
	jobs, _, err := u.nomad.Jobs().List(&nomadApi.QueryOptions{Filter: UpdateableJobsFilterExpr})
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

func (u *Updater) GetJobInfo(jobId string) (*nomadApi.Job, error) {
	job, _, err := u.nomad.Jobs().Info(jobId, &nomadApi.QueryOptions{})
	if err != nil {
		return &nomadApi.Job{}, fmt.Errorf("failed to get info for nomad job '%s': %w", jobId, err)
	}
	slog.Debug("got job info", slog.String("id", jobId), slog.Any("job", job))

	return job, nil
}

func (u *Updater) GetJobSource(jobId string, jobVersion *int) (*nomadApi.JobSubmission, error) {
	if jobVersion == nil {
		slog.Debug("no job source version specified, loading from job info")
		job, err := u.GetJobInfo(jobId)
		if err != nil {
			return &nomadApi.JobSubmission{}, err
		}
		jobVersion = new(int)
		*jobVersion = int(*job.Version)
	}

	slog.Debug("loading job source", slog.Int("version", *jobVersion))
	jobSrc, _, err := u.nomad.Jobs().Submission(jobId, *jobVersion, &nomadApi.QueryOptions{})
	if err != nil {
		return &nomadApi.JobSubmission{}, fmt.Errorf("failed to get source for nomad job '%s': %w", jobId, err)
	}
	slog.Debug("got job source", slog.String("id", jobId), slog.Any("source", jobSrc))

	return jobSrc, nil
}
