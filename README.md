# flyd-mini: FSM-driven OCI image orchestrator

## Overview
flyd-mini is a compact orchestrator that demonstrates how to reliably retrieve container image layers from S3, unpack them into a canonical filesystem layout, place them inside a DeviceMapper thinpool, and activate read-write snapshots for runtime use. The project is built around the superfly/fsm FSM library for durable state transitions and safe concurrency, and uses SQLite for durable metadata storage.

### Key ideas
- Use an FSM to make workflow stages durable, resumable, and explicit.
- Deduplicate S3 blobs by ETag and track them by content digest so repeated pulls are idempotent.
- Unpack image layers into a canonical rootfs/ layout inside a thin logical volume.
- Provide a database-backed lock system so operations survive and behave sensibly in hostile, multi-process environments.
- Track images, blobs, image->blob relationships, and activations in SQLite.

### Important project facts 
- FSM library: https://github.com/superfly/fsm (used to manage states and concurrency).
- S3 source: https://flyio-platform-hiring-challenge.s3.us-east-1.amazonaws.com/ (public bucket, no keys required).
- The S3 bucket contains families such as images/golang/*, images/node/*, images/python/*. Layers within a family are ordered and together make an image.
- Device-mapper backing device snippet used in development:

```bash
# example  snippet to create a thin-pool from terminal
fallocate -l 1M pool_meta
fallocate -l 2G pool_data

METADATA_DEV="$(losetup -f --show pool_meta)"
DATA_DEV="$(losetup -f --show pool_data)"

# create thinpool (example parameters used in testing)
dmsetup create --verifyudev pool --table "0 4194304 thin-pool ${METADATA_DEV} ${DATA_DEV} 2048 32768"
```

Note: these commands require root and Linux tools: fallocate, losetup, dmsetup, mkfs.ext4, mount.

## How it behaves

Each stage of the process is modeled as a distinct FSM state, which means every step is durable, resumable, and isolated. If a crash or interruption occurs, the FSM can safely resume from the last completed state rather than restarting the entire workflow. This also makes it clear how concurrency and ordering are enforced across transitions.

1.	FetchObject stage
	- Acquire DB lock for the image family.
	- Read S3 listing, build a list of layers for the requested family.
	- Use existing etag index to skip already-complete blobs.
	- Download missing layers, compute sha256 digest, store blob metadata and link each blob into image_blobs.
	- Mark image complete only after all blobs are present.
2.	UnpackLayers stage
	- Verify all expected layer files exist.
	- Extract layers in order into rootfs/ (canonical layout). Extraction can use tar -xf wrapper or Go tar reader.
	- Ensure extraction happens into a clean or snapshot-backed mount to avoid “file exists” conflicts.
3.	RegisterImage stage
	- If images.base_lv_id exists, skip creation.
	- Else create thinpool backing files if needed, attach loop devices, create thinpool, allocate a base thin LV (random id validated against DB), create device mapper device, format and mount, copy rootfs/ into the base LV, unmount, and write base_lv_id into images table.
4.	ActivateSnapshot stage
	- Create a snapshot of the base LV in the thinpool.
	- Map it with dmsetup create, mount at /mnt/images/<snap_id>, insert activation row in DB.
5.	Cleanup / safe shutdown
	- Locks are released using a fresh short-lived context so they are cleaned even if the FSM root context is cancelled.
	- Helper cleanup script unmounts /mnt/images/*, removes mapped devices, detaches loop devices and removes backing files.

### Operational notes and constraints
	- Core tool is written in Golang. Shell scripts are used for DM operations where appropriate.
	- The code assumes it may run in a hostile environment. DB-backed locks and idempotent upserts are used to reduce races.
	- The S3 listing and blobs may differ between environments. The code accepts missing blobs in the S3 source and treats them as pending until they appear in test runs.

## Quickstart (development)
1.	Ensure Linux with dmsetup and losetup installed and run as root for the DM steps.
2.	Build binary:

```bash
go build ./cmd/flyd -o flyd
```

3.	Initialize DB and run:

```bash
./flyd
```

4.	Example run: start an FSM run to fetch golang family (the CLI or code calls the FSM start function with appropriate request). The tool writes a results.json with the final snapshot details.

## Notes and caveats
- Device-mapper operations require root privileges and are host destructive if misused. Use the provided cleanup script after testing.

- Concurrency inside a single transition is left conservative to avoid DB races. If you introduce parallel downloads, ensure serialized DB writes or robust retry handling.

- The code is intentionally pragmatic: some DM operations are shell-wrapped for simplicity.
