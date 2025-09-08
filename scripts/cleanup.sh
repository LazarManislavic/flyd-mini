#!/bin/bash
set -euo pipefail

echo "Starting Flyd thinpool cleanup..."

MOUNT_POINT="/mnt/base_lv"

# 1. Unmount any mounted thin volumes under /mnt/base_lv
if mountpoint -q "$MOUNT_POINT"; then
    echo "Unmounting $MOUNT_POINT..."
    sudo umount "$MOUNT_POINT"
else
    echo "$MOUNT_POINT not mounted, skipping."
fi

# 1a. Also check for any other mounts under /mnt/base_lv*
for mp in $(mount | grep '/dev/mapper/base_lv_' | awk '{print $3}'); do
    echo "Unmounting additional mount point $mp..."
    sudo umount "$mp"
done

# 1b. Unmount all mounts under /mnt/images/*
for mp in $(mount | awk '$3 ~ /^\/mnt\/images\// {print $3}'); do
    echo "Unmounting snapshot mount $mp..."
    sudo umount "$mp" || echo "Failed to unmount $mp"
done

# 2. Remove snapshot devices explicitly
for snap in $(sudo dmsetup ls --noheadings | awk '{print $1}' | grep -E '_snap'); do
    echo "Removing snapshot device $snap..."
    sudo dmsetup remove "$snap" || echo "Failed to remove snapshot $snap"
done

# 3. Remove all other thin logical volumes detected by dmsetup
for lv in $(sudo dmsetup ls --noheadings | awk '{print $1}'); do
    if [[ "$lv" != "pool" ]]; then
        echo "Removing thin LV $lv..."
        sudo dmsetup remove "$lv" || echo "Failed to remove $lv"
    fi
done

# 4. Remove the thinpool device
if [ -e /dev/mapper/pool ]; then
    echo "Removing thinpool device /dev/mapper/pool..."
    sudo dmsetup remove pool || echo "Failed to remove pool, check for active thin LVs"
fi

# 5. Detach all loop devices associated with pool_meta and pool_data
echo "Detaching all loop devices associated with pool_meta and pool_data..."
LOOPS=$(losetup -l | awk '/pool_meta|pool_data/ {print $1}' | sort -Vr)
for loop in $LOOPS; do
    if [ -n "$loop" ]; then
        echo "Detaching $loop..."
        sudo losetup -d "$loop" || echo "Failed to detach $loop"
    fi
done

# 6. Remove backing files
for file in pool_meta pool_data; do
    if [ -f "$file" ]; then
        echo "Removing backing file $file..."
        rm -f "$file"
    fi
done

# 7. Remove empty mount point
if [ -d "$MOUNT_POINT" ] && [ -z "$(ls -A "$MOUNT_POINT")" ]; then
    echo "Removing empty mount point $MOUNT_POINT..."
    rmdir "$MOUNT_POINT"
fi

echo "Thinpool, snapshots, and loop device cleanup completed successfully."
