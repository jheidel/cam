# Camera Motion Detection & Recording Project

Because ZoneMinder is burdened by history and doesn't quite do what I want.

Work in progress. Not yet usable.

# Installation

 - Build and install FFMPEG from source

```
git clone https://git.ffmpeg.org/ffmpeg.git ffmpeg
git checkout release/4.0
./configure --enable-gpl --enable-libx264 --enable-shared --enable-static
make -j 4
sudo make install
sudo ldconfig

# Test
ffmpeg -version
```


 - Follow gocv installation (https://github.com/hybridgroup/gocv#how-to-install)
  - Make sure `FFMPEG: YES` during cmake, otherwise `Read` will fail.

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
 - Camera abstraction
 - Configuration file
 - Polymer frontend

Web endpoints:

 /mjpeg
   (mjpeg handler, takes a camera and a debug name)

 /video
   (returns mp4 data directly)

 /thumb
   (returns jpeg thumbnail)

 /vthumb
   (returns mp4 thumbnail)

 /cameras
   (lists camera information, JSON)

 /events
   (lists historical event information, JSON)

 /eventstream
   (save as events, but proivides a streaming update)

Websocket for streaming data?


# Other golang deps

go get github.com/gorilla/websocket
go get github.com/pillash/mp4util
go get github.com/sirupsen/logrus
go get github.com/elazarl/go-bindata-assetfs

# Other needed tools

sudo apt-get install go-bindata
go-bindata web/default/build


# Installing Polymer

```
sudo apt-get install npm

sudo npm install -g polymer-cli
sudo npm install -g bower
sudo apt-get install default-jre
```

