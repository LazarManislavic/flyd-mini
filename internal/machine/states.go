package machine

import (
	"context"
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

	"github.com/manuelinfosec/flyd/internal"
	"github.com/manuelinfosec/flyd/internal/s3"
	"github.com/manuelinfosec/flyd/internal/storage"
	"github.com/sirupsen/logrus"
	"github.com/superfly/fsm"
)

// FetchObject:
//   - Check if image exists locally
//   - Read images by name
//   - If it exists -> skip download
//   - If not -> fetch + hash
//   - Write to blobs & images table
func FetchObject(ctx context.Context, req *fsm.Request[FSMRequest, FSMResponse], app *AppContext) (*fsm.Response[FSMResponse], error) {
	destDir := "blobs"
	logrus.Infof("Starting FetchObject for image family: %s", req.Msg.ImageName)

	// Step 1: Get XML listing
	url := fmt.Sprintf("https://%s.s3.us-east-1.amazonaws.com/", req.Msg.BucketName)
	reqHTTP, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		logrus.Errorf("Failed to create HTTP request: %v", err)
		return nil, err
	}
	resp, err := http.DefaultClient.Do(reqHTTP)
	if err != nil {
		logrus.Errorf("Failed to fetch bucket listing: %v", err)
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		logrus.Errorf("Failed to read bucket listing: %v", err)
		return nil, err
	}

	var listing s3.ListBucketResult
	if err := xml.Unmarshal(data, &listing); err != nil {
		logrus.Errorf("Failed to parse XML listing: %v", err)
		return nil, fmt.Errorf("xml parse failed: %w", err)
	}

	logrus.Infof("Fetched XML listing, processing layers for image: %s", req.Msg.ImageName)

	// Step 2: Fetch layers for the requested family
	seenETags := make(map[string]bool)
	paths, err := s3.FetchImageLayers(ctx, app.S3, req.Msg.ImageName, listing, destDir, seenETags)
	if err != nil {
		logrus.Errorf("Failed to fetch image layers: %v", err)
		return nil, fmt.Errorf("fetch image layers failed: %w", err)
	}
	logrus.Infof("Fetched %d layers for image family %s", len(paths), req.Msg.ImageName)

	// Step 3: Save each blob into DB
	var lastDigest string
	var lastPath string

	for _, p := range paths {
		stat, err := os.Stat(p[0])
		if err != nil {
			logrus.Errorf("Failed to stat %s: %v", p[0], err)
			return nil, fmt.Errorf("failed to stat %s: %w", p[0], err)
		}
		size := stat.Size()

		digest, err := s3.ComputeFileDigest(p[0])
		if err != nil {
			logrus.Errorf("Failed to compute digest for %s: %v", p[0], err)
			return nil, err
		}

		if err := storage.InsertBlob(ctx, app.DB, digest, size, p[0], p[1]); err != nil {
			logrus.Errorf("Failed to insert blob %s: %v", p[0], err)
			return nil, fmt.Errorf("failed to insert blob: %w", err)
		}

		logrus.Infof("Inserted blob %s into database", p[0])
		lastDigest = digest
		lastPath = p[0]
	}

	// Insert image row referencing last blob
	imageRowID, err := storage.InsertImage(ctx, app.DB,
		req.Msg.ImageName, lastDigest, nil, 0, lastPath)
	if err != nil {
		logrus.Errorf("Failed to upsert image row: %v", err)
		return nil, fmt.Errorf("failed to upsert image row: %w", err)
	}
	logrus.Infof("Inserted/updated image row %d for %s", imageRowID, req.Msg.ImageName)

	return &fsm.Response[FSMResponse]{
		Msg: &FSMResponse{
			BaseDir:   "rootfs/",
			ImageID:   imageRowID,
			LocalPath: "blobs/",
		},
	}, nil
}

// UnpackLayers unpacks all tarballs in LocalPath dir into BaseDir
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

func RegisterImage(ctx context.Context, req *fsm.Request[FSMRequest, FSMResponse], app *AppContext) (*fsm.Response[FSMResponse], error) {
	const (
		poolMetaFile = "pool_meta"
		poolDataFile = "pool_data"
		poolDevice   = "/dev/mapper/pool"
		mountRoot    = "/mnt/base_lv"
	)

	logrus.Infof("Starting RegisterImage for ImageID %d", req.W.Msg.ImageID)

	// 1. Check if image already has a base_lv_id
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

	// 2. Prepare backing files if they don't exist
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

	// 3. Attach loop devices
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

	// 4. Create thinpool if it doesn't exist
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

	// 5. Generate a random base LV ID and validate against DB
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

	// 6. Create the thin volume
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

	// 7. Format the thin volume
	logrus.Infof("Formatting thin volume %s", thinDevice)
	if err := exec.Command("mkfs.ext4", thinDevice).Run(); err != nil {
		logrus.Errorf("Failed to format thin volume: %v", err)
		return nil, fmt.Errorf("failed to format base LV: %w", err)
	}

	// 8. Mount and copy rootfs
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

	// 9. Write the base_lv_id back to the image row
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

	// 1. Lookup image row to get base_lv_id
	img, err := storage.GetImageByID(ctx, app.DB, req.W.Msg.ImageID)
	if err != nil {
		return nil, fmt.Errorf("failed to lookup image: %w", err)
	}
	if img == nil || !img.BaseLvID.Valid {
		return nil, fmt.Errorf("image %d has no base_lv_id", req.W.Msg.ImageID)
	}
	baseLvID := img.BaseLvID.Int64

	// 2. Generate snapshot ID (random, validated against DB)
	rand.Seed(time.Now().UnixNano())
	var snapLvID int64
	for {
		snapLvID = int64(rand.Intn(1_000_000) + 1)
		existing, err := storage.GetActivationBySnapLvID(ctx, app.DB, snapLvID)
		if err != nil {
			return nil, fmt.Errorf("failed to check snap_lv_id in DB: %w", err)
		}
		if existing == nil {
			break
		}
	}

	// 3. Create snapshot volume via dmsetup
	if err := exec.Command("dmsetup", "message", poolDevice, "0", fmt.Sprintf("create_snap %d %d", snapLvID, baseLvID)).Run(); err != nil {
		return nil, fmt.Errorf("failed to create snapshot: %w", err)
	}

	snapName := fmt.Sprintf("snap_lv_%d", snapLvID)
	snapDevice := fmt.Sprintf("/dev/mapper/%s", snapName)

	if err := exec.Command("dmsetup", "create", snapName,
		"--table", fmt.Sprintf("0 4194304 thin %s %d", poolDevice, snapLvID),
	).Run(); err != nil {
		return nil, fmt.Errorf("failed to map snapshot device: %w", err)
	}

	// 4. Mount snapshot
	mountPath := fmt.Sprintf("/mnt/images/%d", snapLvID)
	if err := os.MkdirAll(mountPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create mount point: %w", err)
	}
	if err := exec.Command("mount", snapDevice, mountPath).Run(); err != nil {
		return nil, fmt.Errorf("failed to mount snapshot: %w", err)
	}

	// 5. Insert into activations table
	actID, err := storage.InsertActivation(ctx, app.DB, req.W.Msg.ImageID, snapLvID, mountPath)
	if err != nil {
		_ = exec.Command("umount", mountPath).Run()
		return nil, fmt.Errorf("failed to insert activation row: %w", err)
	}

	return &fsm.Response[FSMResponse]{
		Msg: &FSMResponse{
			BaseDir:   mountPath,
			ImageID:   req.W.Msg.ImageID,
			LocalPath: snapDevice,
			SnapshotRef: actID,
		},
	}, nil
}