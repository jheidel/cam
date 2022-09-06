####
# Build the web frontend
####

FROM alpine AS cam-builder-web
WORKDIR /web/
RUN apk add --no-cache nodejs npm make git

# Make sure npm is up to date
RUN npm install -g npm

# Install yarn for web dependency management
RUN npm install -g yarn

# Install polymer CLI
RUN yarn global add polymer-cli

# Copy web source files
COPY web/ .

# Build the frontend
RUN make

####
# Build the go binary
####
FROM gocv/opencv:4.5.5 AS cam-builder-go

WORKDIR /root/go/src/cam/

# Install go-bindata executable
RUN go install github.com/go-bindata/go-bindata/go-bindata@latest
RUN go install github.com/elazarl/go-bindata-assetfs/go-bindata-assetfs@latest

# Copy all source files.
COPY . .

# Copy built web package from the previous stage.
COPY --from=cam-builder-web /web/build/ /root/go/src/cam/web/build/

RUN make build
RUN make libs

####
# Compose everything into the final minimal image.
####

# NOTE: this must match the debian version used by gocv's Docker.opencv
# otherwise the shared library copy completely breaks the system.
FROM debian:buster-slim
WORKDIR /app

# Install dependencies
RUN apt update && apt install -y ffmpeg tzdata ca-certificates
RUN update-ca-certificates
# Clean up apt garbage to keep the image small
RUN apt clean autoclean && apt autoremove -y && rm -rf /var/lib/{apt,dpkg,cache,log}/

# Use local timezone.
# TODO: make image timezone-agnostic (currently used for quiet hours)
RUN ln -fs /usr/share/zoneinfo/America/Los_Angeles /etc/localtime && dpkg-reconfigure -f noninteractive tzdata

# Install application
COPY --from=cam-builder-go /root/go/src/cam/cam /app
# Pull shared libraries for opencv and dependencies
COPY --from=cam-builder-go /root/go/src/cam/libs /usr/local/lib
RUN ldconfig

COPY entrypoint.sh .

EXPOSE 443
EXPOSE 80

ENV GOTRACEBACK=system
ENTRYPOINT ["/app/entrypoint.sh"]
