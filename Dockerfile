####
# Build the web frontend
####

FROM alpine AS cam-builder-web
WORKDIR /web/
RUN apk add --no-cache nodejs nodejs-npm make

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
FROM gocv/opencv:4.5.2 AS cam-builder-go

WORKDIR /root/go/src/cam/

# Install go-bindata executable
RUN go get github.com/go-bindata/go-bindata/...
RUN go get github.com/elazarl/go-bindata-assetfs/...

# Copy all source files.
COPY . .

# Copy built web package from the previous stage.
COPY --from=cam-builder-web /web/build/ /root/go/src/cam/web/build/

RUN make build
RUN make libs

####
# Compose everything into the final minimal image.
####

FROM debian:buster-slim
WORKDIR /app

# Install dependencies
RUN apt update && apt install -y ffmpeg tzdata ca-certificates
RUN update-ca-certificates
# Clean up apt garbage to keep the image small
RUN apt clean autoclean && apt autoremove -y && rm -rf /var/lib/{apt,dpkg,cache,log}/

# Create binding mount points...
# ...for video database
RUN mkdir -p /mnt/db
# ...for configuration
RUN mkdir -p /mnt/config
# ...for HTTP certificates
RUN mkdir -p /etc/letsencrypt

# Use local timezone.
# TODO: make image timezone-agnostic (currently used for quiet hours)
RUN ln -fs /usr/share/zoneinfo/America/Los_Angeles /etc/localtime && dpkg-reconfigure -f noninteractive tzdata

# Install application
COPY --from=cam-builder-go /root/go/src/cam/cam /app
# Pull shared libraries for opencv and dependencies
COPY --from=cam-builder-go /root/go/src/cam/libs /usr/local/lib
RUN ldconfig

EXPOSE 8443
CMD ["./cam", "--port", "8443", "--root", "/mnt/db", "--config", "/mnt/config/config.json"]
