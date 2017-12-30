# Camera Motion Detection & Recording Project

Work in progress. Not yet usable.

# Installation

 - Build and install FFMPEG from source (TODO details)
 - Follow gocv installation: https://github.com/hybridgroup/gocv#how-to-install
  - libjasper needs to be removed for `make deps` to work

# Building

TODO


# Encoding Notes

```
# Final quality
ffmpeg -i /tmp/test.avi -c:v libx264 -preset superfast -movflags +faststart -crf 30 /tmp/test.mp4

# Thumbnail
# Maybe consider webm
ffmpeg -i /tmp/test.avi -r 3 -c:v libx264 -crf 30 -vf "setpts=0.2*PTS,scale=240:135" -preset superfast /tmp/test.mp4
```

Pipe directly instead of temp file? 
https://stackoverflow.com/questions/5825173/pipe-raw-opencv-images-to-ffmpeg

Needs some gocv tweaking.

# TODO

Contribute gocv library changes back to library.
