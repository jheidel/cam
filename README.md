# Camera Motion Detection & Recording Project

Because ZoneMinder is burdened by history and doesn't quite do what I want.

This project is a prototype and might require a bit of work to use.

## Installation

To run, use docker compose.

```
docker-compose up -d
```

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

