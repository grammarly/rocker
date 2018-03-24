# "Interstellar" – PoC of MOUNT alternative with vanilla Docker
#
# This Dockerfile + build.sh illustrates how to keep cache of package managers
# and build systems between different build executions even after RUN layer
# invalidation. This trick help survive without Rocker's MOUNT.
#
# See the discussion https://github.com/grammarly/rocker/issues/199
FROM golang:latest as builder

COPY . /go/src/github.com/grammarly/rocker
WORKDIR /go/src/github.com/grammarly/rocker

# Note that on the first "cold" run the local directory ".cache/go-build" is empty
COPY .cache/go-build /root/.cache/go-build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go install -v

# We use the trick with LABEL to be able to find this layer in build.sh
LABEL rocker_build_cachepoint=true

# Use Docker's multistage build feature to promote only our statically-built rocker binary
FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /go/bin/rocker /bin/rocker
CMD ["/bin/rocker"]