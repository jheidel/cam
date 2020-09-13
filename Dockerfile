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
FROM ubuntu:latest AS cam-builder-go
RUN apt update && apt install -y golang git build-essential sudo

# Manually install gocv
ADD https://api.github.com/repos/jheidel/gocv/git/refs/heads/dev /tmp/version.json
RUN mkdir -p /root/go/src/gocv.io/x/
RUN git clone --branch dev https://github.com/jheidel/gocv.git /root/go/src/gocv.io/x/gocv
WORKDIR /root/go/src/gocv.io/x/gocv/
RUN make deps
RUN make download
RUN make build
RUN make sudo_install
RUN go run ./cmd/version/main.go

WORKDIR /root/go/src/cam/

# Copy all source files.
COPY . .

# Copy built web package from the previous stage.
COPY --from=cam-builder-web /web/build/ /root/go/src/cam/web/build/

# Install go-bindata executable
# TODO(jheidel): This tool is deprecated and it would be a good idea to switch
# onto a maintained go asset package.
RUN apt install -y go-bindata
RUN go get -u github.com/jteeuwen/go-bindata/...

RUN make build
RUN make libs

####
# Compose everything into the final minimal image.
####

#FROM alpine
FROM ubuntu:latest
WORKDIR /app
# Install application
COPY --from=cam-builder-go /root/go/src/cam/cam /app
# Pull shared libraries for opencv and dependencies
COPY --from=cam-builder-go /root/go/src/cam/libs /usr/local/lib
RUN ldconfig

# Install dependencies
RUN apt update && apt install -y ffmpeg tzdata
# Clean up apt garbage to keep the image small
RUN apt clean autoclean && apt autoremove -y && rm -rf /var/lib/{apt,dpkg,cache,log}/

# Create binding mount points...
# ...for video database
RUN mkdir -p /mnt/db
# ...for configuration
RUN mkdir -p /mnt/config
# ...for HTTP certificates
RUN mkdir -p /mnt/cert

# Use local timezone.
# TODO: make image timezone-agnostic (currently used for quiet hours)
RUN ln -fs /usr/share/zoneinfo/America/Los_Angeles /etc/localtime && dpkg-reconfigure -f noninteractive tzdata

EXPOSE 8443
CMD ["./cam", "--port", "8443", "--root", "/mnt/db", "--config", "/mnt/config/config.json", "--cert", "/mnt/cert/fullchain.pem", "--key", "/mnt/cert/privkey.pem"]
