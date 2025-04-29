# Step 1: Define the base image as an argument with a default value
ARG base_image=ubuntu:24.04

# Step 2: Use the official Golang image for building the application
FROM ${base_image}

# Step 3: Install necessary dependencies for the build
RUN apt-get update && apt-get install -y \
  sudo \
  git wget tar curl make jq yq \
  && rm -rf /var/lib/apt/lists/*

# Step 4: Set the working directory and copy the necessary Go files
WORKDIR /app

COPY setup.sh setup.sh

ENV PATH=/usr/local/go/bin:$PATH
RUN chmod +x /app/setup.sh && /app/setup.sh

#COPY ./internal/remote/deployment/createuser ./internal/remote/deployment/
COPY ./go.mod ./go.sum ./

# Step 5: deploymentwnload Go modules
RUN go mod tidy
ENV HOME=/root
ENV GOPATH=$HOME/go
COPY . /app
RUN touch default.yaml && make install

# Step 6: Build the Go application
#RUN go build -v -o ./remote_utils/bin/user-utils ./internal/remote/deployment/createuser
#RUN make build

