package machine

type FSMRequest struct {
	Key       string // S3 key for image tarball
	ImageName string // logical name to register in DB
}

type FSMResponse struct {
	LocalPath   string // where the tarball was downloaded
	BaseDir     string // directory where it was unpacked
	ImageID     int64  // row ID in the images table
	SnapshotRef string // identifier/handle for activated snapshot
}
