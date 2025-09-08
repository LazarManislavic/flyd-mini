package machine

type FSMRequest struct {
	Key       string // S3 key for image tarball
	BucketName string
	ImageName string
}

type FSMResponse struct {
	LocalPath   string // where the tarball was downloaded
	BaseDir     string // directory where it was unpacked
	ImageID     int64  // row ID in the images table
	SnapshotRef int64 // identifier/handle for activated snapshot
}
