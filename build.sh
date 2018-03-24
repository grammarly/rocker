# "Interstellar" – PoC of MOUNT alternative with vanilla Docker
#
# This Dockerfile + build.sh illustrates how to keep cache of package managers
# and build systems between different build executions even after RUN layer
# invalidation. This trick help survive without Rocker's MOUNT.
#
# See the discussion https://github.com/grammarly/rocker/issues/199
set -e

# Make sure there is empty directory during the first run
mkdir -p .cache/go-build

# Do a normal build
docker build -t grammarly/rocker:latest -f Dockerfile .

# Use the label trick to get the ID of the latest layer containing cache of Go's compiler
CACHE_LAYER=`docker images --filter "label=rocker_build_cachepoint=true" --format "{{.ID}}" | head -n 1`

# The next command overwrites any older cache, we may improve it by using `rsync` instead of `cp`
echo "Downloading the latest build cache..."
rm -R .cache/go-build

# Store locally the cache left after the latest build
docker run --rm -ti -v `pwd`/.cache:/parent_cache $CACHE_LAYER \
    /bin/bash -c 'cp -R /root/.cache/go-build /parent_cache/go-build'