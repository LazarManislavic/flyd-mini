#!/bin/bash
set -euo pipefail

echo "Starting Flyd thinpool cleanup..."

MOUNT_POINT="/mnt/base_lv"

# 1. Unmount any mounted thin volumes
if mountpoint -q "$MOUNT_POINT"; then
    echo "Unmounting $MOUNT_POINT..."
    sudo umount "$MOUNT_POINT"
else
    echo "$MOUNT_POINT not mounted, skipping."
fi

# Also check for any other mounts under /mnt/base_lv*
for mp in $(mount | grep '/dev/mapper/base_lv_' | awk '{print $3}'); do
    echo "Unmounting additional mount point $mp..."
    sudo umount "$mp"
done

# 2. Remove all thin logical volumes detected by dmsetup
for lv in $(sudo dmsetup ls --noheadings | awk '{print $1}'); do
    if [[ "$lv" != "pool" ]]; then
        echo "Removing thin LV $lv..."
        sudo dmsetup remove "$lv" || echo "Failed to remove $lv"
    fi
done

# 3. Remove the thinpool device
if [ -e /dev/mapper/pool ]; then
    echo "Removing thinpool device /dev/mapper/pool..."
    sudo dmsetup remove pool || echo "Failed to remove pool, check for active thin LVs"
fi

# 4. Detach all loop devices associated with pool_meta and pool_data
echo "Detaching all loop devices associated with pool_meta and pool_data..."
# Collect all loop devices related to pool_meta/pool_data, sort numerically descending
LOOPS=$(losetup -l | awk '/pool_meta|pool_data/ {print $1}' | sort -Vr)
for loop in $LOOPS; do
    if [ -n "$loop" ]; then
        echo "Detaching $loop..."
        sudo losetup -d "$loop" || echo "Failed to detach $loop"
    fi
done

# 5. Remove backing files
for file in pool_meta pool_data; do
    if [ -f "$file" ]; then
        echo "Removing backing file $file..."
        rm -f "$file"
    fi
done

# 6. Remove empty mount point
if [ -d "$MOUNT_POINT" ] && [ -z "$(ls -A "$MOUNT_POINT")" ]; then
    echo "Removing empty mount point $MOUNT_POINT..."
    rmdir "$MOUNT_POINT"
fi

echo "Thinpool and loop device cleanup completed successfully."
