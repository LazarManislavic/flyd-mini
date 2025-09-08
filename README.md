# Platform Machines Work Sample

This is the only programming challenge we're going to give you for this role. It's very challenging and the bar for it is high.

At https://github.com/superfly/fsm you'll find the FSMv2 library that's at the heart of flyd, our orchestrator. You'll need to figure out what it is, how it works, and how to use it.

At `s3://flyio-platform-hiring-challenge/images` you will find a series of container images in an ad-hoc tarball format.

What we need you to do:
- Use the FSM library, ✅
- to retrieve an arbitrary image from that S3 bucket, ✅
- only if it hasn't been retrieved already, ✅
- unpack the image into a canonical filesystem layout, ✅
- inside of a devicemapper thinpool device, ✅
- again, only if that hasn't already been done, ✅
- then, to "activate" the image, create a snapshot of that thinpool device, ✅
- using SQLite to track the available images. ✅

**This is a simplified model of what our `flyd` orchestrator actually does.**

Because you are building a model of `flyd`, two very important notes:

1. Think carefully about the blobs you're pulling from S3 and what they mean. ✅
2. Assume we are going to run your code in a hostile environment (on a fleet with thousands of physical servers, there's always a hostile environment somewhere). Code accordingly. Note: we do not care about your tests. Do assume though that your code will see blobs that are not currently on S3 (but will be in our test environment). ✅

We intentionally have very little advice or documentation to provide for this challenge. Several of us have run through it. The one snag we'll save you from:
```
fallocate -l 1M pool_meta
fallocate -l 2G pool_data

METADATA_DEV="$(losetup -f --show pool_meta)"
DATA_DEV="$(losetup -f --show pool_data)"

dmsetup create --verifyudev pool --table "0 4194304 thin-pool ${METADATA_DEV} ${DATA_DEV} 2048 32768"
```

The requirements:

- We're not timing you (but this should take a couple hours, not days).
- You can write and call out to shell scripts, but the core tool needs to be in Golang.
- We will not evaluate your tests.
- Use whichever libraries you like.
- We can't offer much help or answer many questions (but, rest assured: there is not just one "shape" a successful response can take, as long as it uses fsm, downloads from S3, sets up a SQLite database, and uses DeviceMapper).
- You can use an LLM to help you with any part of this. We've run this challenge through an LLM. Your LLM mileage may vary from ours, but: the bar for this challenge is higher than what we've gotten Claude, 4.5, or o3 to cough up.

This challenge is kind of a lot. But it's our only tech-out challenge, and our whole hiring process shouldn't eat more time than you'd spend in a normal interview loop. More importantly: if doing this is a slog, rather than a nerdsnipe, this isn't a good role fit.
