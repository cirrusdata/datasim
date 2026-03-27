# Running datasim with Docker

datasim provides native binaries for Linux and Windows. If you are on macOS or prefer not to install a binary directly, you can run datasim in a Docker container.

## Quick start

Build the image:
```bash
docker build -t datasim .
```

Run the fileset simulation:
```bash
docker run -v $(pwd)/data:/data datasim fileset init --fs /data --profile corporate --size 1GiB
docker run -v $(pwd)/data:/data datasim fileset status /data
docker run -v $(pwd)/data:/data datasim fileset rotate --fs /data
docker run -v $(pwd)/data:/data datasim fileset destroy /data
```

Or use Docker Compose to avoid repeating the volume mount:
```bash
docker compose run datasim fileset init --fs /data --profile corporate --size 1GiB
docker compose run datasim fileset status /data
docker compose run datasim fileset rotate --fs /data
docker compose run datasim fileset destroy /data
```

To run a rotate loop in the background:
```bash
docker compose run -d datasim fileset rotate loop --fs /data --interval 5m
```

To stop the loop:
```bash
docker ps          # find the container ID
docker stop <id>
```

## Notes

**Always pass `--size` explicitly.** When `--size` is omitted, datasim targets 80% of the filesystem's reported capacity. Inside a container, that capacity comes from the Docker VM's virtual disk, not your host machine's storage. Passing `--size` avoids unexpected behavior.

**`block-device` is not available.** The `block-device` subcommand formats and mounts raw disks and partitions, which requires direct access to host block devices. Containers use mounted filesystems by default, so `block-device` is unnecessary — just point `--fs` at your volume mount.

**Self-update does not persist.** Running `datasim update` inside a container replaces the binary in the container's ephemeral filesystem. Rebuild the image to pick up new releases instead.