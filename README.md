# Camera Motion Detection & Recording Project

[![Build Status](https://app.travis-ci.com/jheidel/cam.svg?branch=master)](https://app.travis-ci.com/jheidel/cam)

Because ZoneMinder is burdened by history and doesn't quite do what I want.

This application supports streaming and recording from any video source
supported by OpenCV (including an MJPEG stream), basic motion detection,
recording of motion events to MP4 files, object detection, and notifications.
It includes a web frontend for showing an activity log.

Built using [Go](https://go.dev/), [OpenCV](https://opencv.org/) (bindings
provided by [GoCV](https://gocv.io/)), [ffmpeg](https://ffmpeg.org/), and
[Polymer](https://polymer-library.polymer-project.org/).

NOTE: This project is a prototype and might require a bit of work to use.

## Setup

Deployment is done using a docker container.

First, copy `config.template.json` to `config.json` in a new directory and edit
as appropriate.

Next, edit the [docker compose](https://docs.docker.com/compose/) file as
needed and deploy:

```
docker-compose up -d
```

A continuous build image is provided on [docker hub](https://hub.docker.com/r/jheidel/cam).

## Development

TODO: add instructions for building and running locally.

## NOTES

 - Come up with a name for this project
 - Support multiple camera inputs

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

TODO: list of endpoints is incomplete

