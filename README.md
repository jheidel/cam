# Camera Motion Detection & Recording Project

Because ZoneMinder is burdened by history and doesn't quite do what I want.

Work in progress. Not yet usable.

# Installation

 - Build and install FFMPEG from source
  - TODO: details
 - Follow gocv installation (https://github.com/hybridgroup/gocv#how-to-install)
  - libjasper needs to be removed from Makefile for `make deps` to work on
    Ubuntu 17.10

# Building

Need to run `source $GOROOT/src/gocv.io/x/gocv/env.sh` before doing `go build` for the OpenCV C bindings to be picked up correctly.

TODO: check out bazel

# Encoding Notes

```
# Final quality
ffmpeg -i /tmp/test.avi -c:v libx264 -preset superfast -movflags +faststart -crf 30 /tmp/test.mp4

# Thumbnail
# Maybe consider webm; though will need recompiled FFmpeg.
ffmpeg -i /tmp/test.avi -r 3 -c:v libx264 -crf 30 -vf "setpts=0.2*PTS,scale=240:135" -preset superfast /tmp/test.mp4
```

Pipe directly instead of temp file (DONE, works great)
https://stackoverflow.com/questions/5825173/pipe-raw-opencv-images-to-ffmpeg

# Test mjpeg stream

http://quadcam.unrfound.unr.edu/axis-cgi/mjpg/video.cgi

# TODO

 - Contribute gocv library changes back to library (In progress!)
 - Come up with a name for this project.

