package machine

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/manuelinfosec/flyd/internal/s3"
	"github.com/manuelinfosec/flyd/internal/storage"
	internal "github.com/manuelinfosec/flyd/internal/util"
	"github.com/sirupsen/logrus"
	"github.com/superfly/fsm"
)

// FetchObject:
//   - Acquire a lock to avoid race conditions when multiple processes fetch the same image.
//   - Check if the image already exists locally in the `images` table.
//   - If it exists and is marked complete -> skip fetching from S3.
//   - If it exists but incomplete -> continue fetching missing blobs.
//   - Download and hash any missing blobs from S3 (based on ETags).
//   - Insert blobs into the `blobs` table (with a completion flag).
//   - Link blobs to the image via `image_blobs` table.
//   - Insert or update the `images` table entry.
//   - Update the image completion flag depending on whether all blobs are present.
//   - Return the image ID, base directory, and blob path for downstream transitions.
func FetchObject(ctx context.Context, req *fsm.Request[FSMRequest, FSMResponse], app *AppContext) (*fsm.Response[FSMResponse], error) {
	destDir := "blobs"
	logrus.Infof("Starting FetchObject for image family: %s", req.Msg.ImageName)

	// lock mechanism to avoid two processes trying to fetch/write the same image simultaneously.
	lockKey := fmt.Sprintf("fetch:%s", req.Msg.ImageName)
	lockVal := fmt.Sprintf("pid-%d", os.Getpid())

	locked, err := storage.AcquireLock(ctx, app.DB, lockKey, lockVal, 30*time.Second)
	if err != nil {
		logrus.Errorf("Error while trying to acquire lock for %s: %v", req.Msg.ImageName, err)
		return nil, err
	}
	if !locked {
		logrus.Warnf("Could not acquire lock for image %s, skipping fetch", req.Msg.ImageName)
		return nil, fmt.Errorf("lock contention for %s", req.Msg.ImageName)
	}
	defer func() {
		// A different context to release lock because the global context is usually
		// canclled during shutdown before some locks are released.
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		// Always release lock
		if err := storage.ReleaseLock(ctx, app.DB, lockKey, lockVal); err != nil {
			logrus.Warnf("Failed to release lock %s: %v", lockKey, err)
		} else {
			logrus.Infof("Released lock %s", lockKey)
		}
	}()

	// Create the `blobs/` directory if it doesn't exist
	if err := storage.EnsureDir("blobs"); err != nil {
		logrus.Fatalf("Failed to ensure blobs directory: %v", err)
	}

	// Get XML listing
	url := fmt.Sprintf("https://%s.s3.us-east-1.amazonaws.com/", req.Msg.BucketName)
	reqHTTP, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}
	resp, err := http.DefaultClient.Do(reqHTTP)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch bucket listing: %w", err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read bucket listing: %w", err)
	}

	var listing s3.ListBucketResult
	if err := xml.Unmarshal(data, &listing); err != nil {
		return nil, fmt.Errorf("xml parse failed: %w", err)
	}

	logrus.Infof("Fetched XML listing, processing layers for image: %s", req.Msg.ImageName)

	// Fetch layers for the requested family but skip if blob already exists
	seenETags, err := storage.GetAllETags(ctx, app.DB)
	if err != nil {
		return nil, fmt.Errorf("failed to load seen etags: %w", err)
	}
	paths, err := s3.FetchImageLayers(ctx, app.S3, req.Msg.ImageName, listing, destDir, seenETags)
	if err != nil {
		return nil, fmt.Errorf("fetch image layers failed: %w", err)
	}
	logrus.Infof("Fetched %d layers (new or existing) for image family %s", len(paths), req.Msg.ImageName)

	// Save blobs into DB and link to image
	var lastDigest string
	var lastPath string

	for _, p := range paths {
		stat, err := os.Stat(p[0])
		if err != nil {
			// If the file doesn't exist locally, still link blob digest to image, mark incomplete
			logrus.Warnf("Blob file missing for %s, will mark as incomplete", p[1])
			blobDigest := p[1]
			_ = storage.InsertBlob(ctx, app.DB, blobDigest, 0, "", p[1], false)
			continue
		}
		size := stat.Size()

		digest, err := s3.ComputeFileDigest(p[0])
		if err != nil {
			return nil, fmt.Errorf("failed to compute digest for %s: %w", p[0], err)
		}

		if err := storage.InsertBlob(ctx, app.DB, digest, size, p[0], p[1], true); err != nil {
			return nil, fmt.Errorf("failed to insert blob: %w", err)
		}

		lastDigest = digest
		lastPath = p[0]
	}

	// Insert or update image row
	imageRowID, err := storage.InsertImage(ctx, app.DB,
		req.Msg.ImageName, lastDigest, nil, 0, lastPath)
	if err != nil {
		return nil, fmt.Errorf("failed to upsert image row: %w", err)
	}

	// Link blobs to the image
	for _, p := range paths {
		blobDigest := p[1]
		if err := storage.InsertImageBlob(ctx, app.DB, imageRowID, blobDigest); err != nil {
			return nil, fmt.Errorf("failed to insert image->blob mapping: %w", err)
		}
	}

	// Update image completion flag
	if err := storage.UpdateImageCompletion(ctx, app.DB, imageRowID); err != nil {
		return nil, fmt.Errorf("failed to update image completion: %w", err)
	}

	logrus.Infof("Inserted/updated image row %d for %s (completion updated)", imageRowID, req.Msg.ImageName)

	return &fsm.Response[FSMResponse]{
		Msg: &FSMResponse{
			BaseDir:     "rootfs/",
			ImageID:     imageRowID,
			LocalPath:   "blobs/",
			SnapshotRef: 0,
		},
	}, nil
}

// UnpackLayers:
//   - Ensure the destination directory (`BaseDir`) exists.
//   - Iterate through tarball layers belonging to the image in `LocalPath`.
//   - For each layer, unpack it into the canonical filesystem layout under `BaseDir`.
//   - Preserve file attributes and merge layers in the correct order.
//   - Return an FSM response with updated `BaseDir`, `ImageID`, and `LocalPath`
//     for downstream transitions (e.g., RegisterImage).
func UnpackLayers(ctx context.Context, req *fsm.Request[FSMRequest, FSMResponse], app *AppContext) (*fsm.Response[FSMResponse], error) {
	srcDir := req.W.Msg.LocalPath
	destDir := req.W.Msg.BaseDir

	logrus.Infof("Starting UnpackLayers from %s to %s", srcDir, destDir)

	if err := os.MkdirAll(destDir, 0755); err != nil {
		logrus.Errorf("Failed to create base dir %s: %v", destDir, err)
		return nil, fmt.Errorf("failed to create base dir: %w", err)
	}

	for i := 1; i <= 5; i++ {
		tarPath := filepath.Join(srcDir, fmt.Sprintf("images_%s_%d.tar", req.Msg.ImageName, i))
		if _, err := os.Stat(tarPath); err == nil {
			logrus.Infof("Unpacking %s into %s", tarPath, destDir)
			if err := internal.UnpackTar(tarPath, destDir); err != nil {
				logrus.Errorf("Failed to unpack %s: %v", tarPath, err)
				return nil, fmt.Errorf("failed to unpack %s: %w", tarPath, err)
			}
		} else {
			logrus.Infof("Layer %s not found, skipping", tarPath)
		}
	}

	logrus.Infof("Finished unpacking layers into %s", destDir)

	return &fsm.Response[FSMResponse]{
		Msg: &FSMResponse{
			BaseDir:   destDir,
			ImageID:   req.W.Msg.ImageID,
			LocalPath: srcDir,
		},
	}, nil
}

// RegisterImage:
//   - Check if the image already has an associated base logical volume (`base_lv_id`).
//     If yes, skip re-registration and return existing device info.
//   - If not, prepare and initialize the device-mapper thinpool if necessary.
//   - Allocate a new thin logical volume ID and ensure it is unique in the database.
//   - Create, format, and mount the thin volume as a base filesystem.
//   - Copy the fully unpacked rootfs from `BaseDir` into the mounted thin volume.
//   - Unmount the volume after the copy completes.
//   - Persist the generated `base_lv_id` back to the `images` table.
//   - Return an FSM response with the thin device path and updated metadata
//     for downstream transitions (e.g., ActivateSnapshot).
func RegisterImage(ctx context.Context, req *fsm.Request[FSMRequest, FSMResponse], app *AppContext) (*fsm.Response[FSMResponse], error) {
	const (
		poolMetaFile = "pool_meta"
		poolDataFile = "pool_data"
		poolDevice   = "/dev/mapper/pool"
		mountRoot    = "/mnt/base_lv"
	)

	logrus.Infof("Starting RegisterImage for ImageID %d", req.W.Msg.ImageID)

	lockKey := fmt.Sprintf("register:%s", req.Msg.ImageName)
	lockVal := fmt.Sprintf("pid-%d", os.Getpid())

	locked, err := storage.AcquireLock(ctx, app.DB, lockKey, lockVal, 30*time.Second)
	if err != nil {
		logrus.Errorf("Error while trying to acquire lock for %s: %v", req.Msg.ImageName, err)
		return nil, err
	}
	if !locked {
		logrus.Warnf("Could not acquire lock for image %s, skipping fetch", req.Msg.ImageName)
		return nil, fmt.Errorf("lock contention for %s", req.Msg.ImageName)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		// Always release lock
		if err := storage.ReleaseLock(ctx, app.DB, lockKey, lockVal); err != nil {
			logrus.Warnf("Failed to release lock %s: %v", lockKey, err)
		} else {
			logrus.Infof("Released lock %s", lockKey)
		}
	}()

	// Check if image already has a base_lv_id
	existingImage, err := storage.GetImageByID(ctx, app.DB, req.W.Msg.ImageID)
	if err != nil {
		logrus.Errorf("Failed to check existing image: %v", err)
		return nil, fmt.Errorf("failed to check existing image: %w", err)
	}
	if existingImage != nil && existingImage.BaseLvID.Valid {
		logrus.Infof("Image already has a base_lv_id %d, skipping thinpool creation", existingImage.BaseLvID.Int64)
		return &fsm.Response[FSMResponse]{
			Msg: &FSMResponse{
				BaseDir:   mountRoot,
				ImageID:   req.W.Msg.ImageID,
				LocalPath: fmt.Sprintf("/dev/mapper/base_lv_%d", existingImage.BaseLvID.Int64),
			},
		}, nil
	}

	// Prepare backing files if they don't exist
	logrus.Infof("Preparing thinpool backing files")
	if _, err := os.Stat(poolMetaFile); os.IsNotExist(err) {
		logrus.Infof("Creating %s", poolMetaFile)
		if err := exec.Command("fallocate", "-l", "1M", poolMetaFile).Run(); err != nil {
			logrus.Errorf("Failed to create pool_meta: %v", err)
			return nil, fmt.Errorf("failed to create pool_meta: %w", err)
		}
	}
	if _, err := os.Stat(poolDataFile); os.IsNotExist(err) {
		logrus.Infof("Creating %s", poolDataFile)
		if err := exec.Command("fallocate", "-l", "2G", poolDataFile).Run(); err != nil {
			logrus.Errorf("Failed to create pool_data: %v", err)
			return nil, fmt.Errorf("failed to create pool_data: %w", err)
		}
	}

	// Attach loop devices
	metaLoopBytes, err := exec.Command("losetup", "-f", "--show", poolMetaFile).Output()
	if err != nil {
		logrus.Errorf("Failed to attach pool_meta: %v", err)
		return nil, fmt.Errorf("failed to attach pool_meta: %w", err)
	}
	metaLoop := strings.TrimSpace(string(metaLoopBytes))

	dataLoopBytes, err := exec.Command("losetup", "-f", "--show", poolDataFile).Output()
	if err != nil {
		logrus.Errorf("Failed to attach pool_data: %v", err)
		return nil, fmt.Errorf("failed to attach pool_data: %w", err)
	}
	dataLoop := strings.TrimSpace(string(dataLoopBytes))

	logrus.Infof("Attached loop devices: meta=%s, data=%s", string(metaLoop), string(dataLoop))

	// Create thinpool if it doesn't exist
	if _, err := os.Stat(poolDevice); os.IsNotExist(err) {
		logrus.Infof("Creating thinpool %s", poolDevice)
		args := []string{
			"create", "--verifyudev", "pool",
			"--table", fmt.Sprintf("0 4194304 thin-pool %s %s 2048 32768", string(metaLoop), string(dataLoop)),
		}
		if err := exec.Command("dmsetup", args...).Run(); err != nil {
			logrus.Errorf("Failed to create thinpool: %v", err)
			return nil, fmt.Errorf("failed to create thinpool: %w", err)
		}
	}

	// Generate a random base LV ID and validate against DB
	logrus.Infof("Generating random base LV ID")
	rand.Seed(time.Now().UnixNano())
	var baseLvID int64
	for {
		baseLvID = int64(rand.Intn(1_000_000) + 1)
		existing, err := storage.GetImageByBaseLvID(ctx, app.DB, baseLvID)
		if err != nil {
			logrus.Errorf("Failed to check base_lv_id in DB: %v", err)
			return nil, fmt.Errorf("failed to check base_lv_id in DB: %w", err)
		}
		if existing == nil {
			break
		}
	}
	logrus.Infof("Selected base_lv_id=%d", baseLvID)

	// Create the thin volume
	logrus.Infof("Creating thin volume with ID %d", baseLvID)
	if err := exec.Command("dmsetup", "message", poolDevice, "0", fmt.Sprintf("create_thin %d", baseLvID)).Run(); err != nil {
		logrus.Errorf("Failed to create thin volume: %v", err)
		return nil, fmt.Errorf("failed to create thin volume: %w", err)
	}

	thinDevice := fmt.Sprintf("/dev/mapper/base_lv_%d", baseLvID)
	if err := exec.Command("dmsetup", "create", fmt.Sprintf("base_lv_%d", baseLvID),
		"--table", fmt.Sprintf("0 4194304 thin %s %d", poolDevice, baseLvID),
	).Run(); err != nil {
		logrus.Errorf("Failed to create device-mapper LV: %v", err)
		return nil, fmt.Errorf("failed to create device-mapper LV: %w", err)
	}

	// Format the thin volume
	logrus.Infof("Formatting thin volume %s", thinDevice)
	if err := exec.Command("mkfs.ext4", thinDevice).Run(); err != nil {
		logrus.Errorf("Failed to format thin volume: %v", err)
		return nil, fmt.Errorf("failed to format base LV: %w", err)
	}

	// Mount and copy rootfs
	logrus.Infof("Mounting thin volume %s to %s", thinDevice, mountRoot)
	if err := os.MkdirAll(mountRoot, 0755); err != nil {
		logrus.Errorf("Failed to create mount point: %v", err)
		return nil, fmt.Errorf("failed to create mount point: %w", err)
	}
	if err := exec.Command("mount", thinDevice, mountRoot).Run(); err != nil {
		logrus.Errorf("Failed to mount thin volume: %v", err)
		return nil, fmt.Errorf("failed to mount base LV: %w", err)
	}

	srcDir := req.W.Msg.BaseDir
	logrus.Infof("Copying rootfs from %s to %s", srcDir, mountRoot)
	if err := exec.Command("cp", "-a", filepath.Join(srcDir, "."), mountRoot).Run(); err != nil {
		_ = exec.Command("umount", mountRoot).Run()
		logrus.Errorf("Failed to copy rootfs: %v", err)
		return nil, fmt.Errorf("failed to copy rootfs: %w", err)
	}
	if err := exec.Command("umount", mountRoot).Run(); err != nil {
		logrus.Errorf("Failed to unmount after copy: %v", err)
		return nil, fmt.Errorf("failed to unmount after copy: %w", err)
	}

	// Write the base_lv_id back to the image row
	if err := storage.UpdateBaseLvID(ctx, app.DB, req.W.Msg.ImageID, baseLvID); err != nil {
		logrus.Errorf("Failed to update base_lv_id in DB: %v", err)
		return nil, fmt.Errorf("failed to update base_lv_id in DB: %w", err)
	}
	logrus.Infof("Completed RegisterImage for ImageID %d with base_lv_id %d", req.W.Msg.ImageID, baseLvID)

	return &fsm.Response[FSMResponse]{
		Msg: &FSMResponse{
			BaseDir:   mountRoot,
			ImageID:   req.W.Msg.ImageID,
			LocalPath: thinDevice,
		},
	}, nil
}

// ActivateSnapshot:
//   - Creates a snapshot of the base LV for the given image
//   - Mounts it under /mnt/images/<activation_id>
//   - Inserts a row into the activations table
func ActivateSnapshot(ctx context.Context, req *fsm.Request[FSMRequest, FSMResponse], app *AppContext) (*fsm.Response[FSMResponse], error) {
	const poolDevice = "/dev/mapper/pool"

	logrus.Infof("Starting ActivateSnapshot for thin device: %s", req.W.Msg.LocalPath)

	lockKey := fmt.Sprintf("activate:%s", req.Msg.ImageName)
	lockVal := fmt.Sprintf("pid-%d", os.Getpid())

	locked, err := storage.AcquireLock(ctx, app.DB, lockKey, lockVal, 30*time.Second)
	if err != nil {
		logrus.Errorf("Error while trying to acquire lock for %s: %v", req.Msg.ImageName, err)
		return nil, err
	}
	if !locked {
		logrus.Warnf("Could not acquire lock for image %s, skipping fetch", req.Msg.ImageName)
		return nil, fmt.Errorf("lock contention for %s", req.Msg.ImageName)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		// Always release lock
		if err := storage.ReleaseLock(ctx, app.DB, lockKey, lockVal); err != nil {
			logrus.Warnf("Failed to release lock %s: %v", lockKey, err)
		} else {
			logrus.Infof("Released lock %s", lockKey)
		}
	}()

	// Lookup image row to get base_lv_id
	logrus.Infof("Looking up image %d in DB...", req.W.Msg.ImageID)
	img, err := storage.GetImageByID(ctx, app.DB, req.W.Msg.ImageID)
	if err != nil {
		logrus.Errorf("DB lookup failed: %v", err)
		return nil, fmt.Errorf("failed to lookup image: %w", err)
	}
	if img == nil || !img.BaseLvID.Valid {
		logrus.Errorf("Image %d has no base_lv_id", req.W.Msg.ImageID)
		return nil, fmt.Errorf("image %d has no base_lv_id", req.W.Msg.ImageID)
	}
	baseLvID := img.BaseLvID.Int64
	logrus.Infof("Found base_lv_id=%d for image %d", baseLvID, req.W.Msg.ImageID)

	// Generate snapshot ID (random, validated against DB)
	rand.Seed(time.Now().UnixNano())
	var snapLvID int64
	for {
		snapLvID = int64(rand.Intn(1_000_000) + 1)
		logrus.Infof("Generated candidate snap_lv_id=%d", snapLvID)
		existing, err := storage.GetActivationBySnapLvID(ctx, app.DB, snapLvID)
		if err != nil {
			logrus.Errorf("DB check for snap_lv_id %d failed: %v", snapLvID, err)
			return nil, fmt.Errorf("failed to check snap_lv_id in DB: %w", err)
		}
		if existing == nil {
			logrus.Infof("snap_lv_id=%d is available", snapLvID)
			break
		}
		logrus.Warnf("snap_lv_id=%d already exists in DB, retrying...", snapLvID)
	}

	// Create snapshot volume via dmsetup
	logrus.Infof("Creating snapshot in thinpool: base_lv_id=%d snap_lv_id=%d", baseLvID, snapLvID)
	if err := exec.Command("dmsetup", "message", poolDevice, "0",
		fmt.Sprintf("create_snap %d %d", snapLvID, baseLvID)).Run(); err != nil {
		logrus.Errorf("dmsetup create_snap failed: %v", err)
		return nil, fmt.Errorf("failed to create snapshot: %w", err)
	}

	snapName := fmt.Sprintf("snap_lv_%d", snapLvID)
	snapDevice := fmt.Sprintf("/dev/mapper/%s", snapName)
	logrus.Infof("Mapping snapshot device: name=%s path=%s", snapName, snapDevice)

	if err := exec.Command("dmsetup", "create", snapName,
		"--table", fmt.Sprintf("0 4194304 thin %s %d", poolDevice, snapLvID),
	).Run(); err != nil {
		logrus.Errorf("dmsetup create failed for %s: %v", snapName, err)
		return nil, fmt.Errorf("failed to map snapshot device: %w", err)
	}

	// Mount snapshot
	mountPath := fmt.Sprintf("/mnt/images/%d", snapLvID)
	logrus.Infof("Creating mount point %s", mountPath)
	if err := os.MkdirAll(mountPath, 0755); err != nil {
		logrus.Errorf("Failed to create mount point %s: %v", mountPath, err)
		return nil, fmt.Errorf("failed to create mount point: %w", err)
	}
	logrus.Infof("Mounting snapshot device %s at %s", snapDevice, mountPath)
	if err := exec.Command("mount", snapDevice, mountPath).Run(); err != nil {
		logrus.Errorf("Failed to mount snapshot device %s: %v", snapDevice, err)
		return nil, fmt.Errorf("failed to mount snapshot: %w", err)
	}

	// Insert into activations table
	logrus.Infof("Inserting activation row: image_id=%d snap_lv_id=%d mount_path=%s",
		req.W.Msg.ImageID, snapLvID, mountPath)
	actID, err := storage.InsertActivation(ctx, app.DB, req.W.Msg.ImageID, snapLvID, mountPath)
	if err != nil {
		logrus.Errorf("Failed to insert activation row: %v", err)
		_ = exec.Command("umount", mountPath).Run()
		return nil, fmt.Errorf("failed to insert activation row: %w", err)
	}
	logrus.Infof("Activation row inserted with ID %d", actID)

	logrus.Infof("ActivateSnapshot complete: image_id=%d snap_lv_id=%d device=%s mounted_at=%s",
		req.W.Msg.ImageID, snapLvID, snapDevice, mountPath)

	return &fsm.Response[FSMResponse]{
		Msg: &FSMResponse{
			BaseDir:     mountPath,
			ImageID:     req.W.Msg.ImageID,
			LocalPath:   snapDevice,
			SnapshotRef: actID,
		},
	}, nil
}

// WriteResults writes the FSMResponse into results.json
func WriteResults(ctx context.Context, req *fsm.Request[FSMRequest, FSMResponse], app *AppContext) (*fsm.Response[FSMResponse], error) {
	outputFile := "results.json"

	// Marshal FSMResponse into JSON
	data, err := json.MarshalIndent(req.W.Msg, "", "  ")
	if err != nil {
		logrus.Errorf("Failed to marshal FSMResponse: %v", err)
		return nil, fmt.Errorf("failed to marshal FSMResponse: %w", err)
	}

	// Write to file
	if err := os.WriteFile(outputFile, data, 0644); err != nil {
		logrus.Errorf("Failed to write results.json: %v", err)
		return nil, fmt.Errorf("failed to write results.json: %w", err)
	}

	logrus.Infof("FSMResponse successfully written to %s", outputFile)

	// Nothing new to return, just forward the same message
	return &fsm.Response[FSMResponse]{
		Msg: req.W.Msg,
	}, nil
}
