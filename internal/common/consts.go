package common

const (
	// Meta property designating a job task as updatable.
	//
	// The value is the current image tag or hash to be used, and will
	// be replaced with the new image hash when the image is updated.
	UpdateableTaskMetaTarget = "autoupdate_imgtag_target"

	// Meta property specifying the corresponding source image tag.
	//
	// The value of this property is used as the source image tag to
	// monitor for hash updates. If this property is not defined, it
	// will be added with its value set to the original value of the
	// target property.
	UpdateableTaskMetaSource = "autoupdate_imgtag_source"

	// Filter expression to filter for jobs with a target meta property defined.
	UpdateableJobsFilterExpr = "any TaskGroups as tg { any tg.Tasks as t" +
		" { " + UpdateableTaskMetaTarget + " in t.Meta } }"
)
