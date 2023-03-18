# List release and operator index details

## Overview

A simple command line interface to list details in both operator and release index images

Used together with the mirror tool will assist in downlaoding selected operator images

## Usage

List release images in a given image index and version

```bash

# list all release images

list release --image-index  quay.io/openshift-release-dev/ocp-release:4.12.0-x86_64 --loglevel trace

```

List operator images in a given image index and version

```bash

# list all operator images

build/list operator --image-index  registry.redhat.io/redhat/redhat-operator-index:v4.12 --loglevel trace

```

