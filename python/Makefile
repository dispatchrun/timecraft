.PHONY: docker-build docker-buildx build-docker build-script clean download

TIMECRAFT_VERSION ?= "main-1687814584-4c9ed9ce"

ifeq ($(GITHUB_BRANCH_NAME),)
	branch := $(shell git rev-parse --abbrev-ref HEAD)-
else
	branch := $(GITHUB_BRANCH_NAME)-
endif
commit_timestamp := $(shell git show --no-patch --format=%ct)-
ifeq ($(GITHUB_SHA),)
	commit := $(shell git rev-parse --short=8 HEAD)
	commit.long := $(shell git rev-parse HEAD)
else
	commit := $(shell echo $(GITHUB_SHA) | cut -c1-8)
	commit.long := $(GITHUB_SHA)
endif
container.version ?= $(if $(RELEASE_TAG),$(RELEASE_TAG),$(shell git describe --tags || echo '$(subst /,-,$(branch))$(commit_timestamp)$(commit)'))
container.image ?= ghcr.io/stealthrocket/timecraft-python

python.commit.long = $(git log --format=format:%H -n1 -- .)

docker-build: 
	docker build -f Dockerfile.timecraft -t $(container.image):$(container.version) .

docker-buildx:
	docker buildx build --push \
		--platform=linux/amd64,linux/arm64 \
		--build-arg TIMECRAFT_VERSION=$(TIMECRAFT_VERSION) \
		-f Dockerfile.timecraft \
		-t $(container.image):$(container.version) .

build-docker:
	docker build -t timecraft-python-builder .
	docker run -v `pwd`:/src -w /src timecraft-python-builder ./build.sh

build-script:
	./build.sh

download:
	./download.sh "$(python.commit.long)"

clean:
	rm -f cpython/python.wasm cpython/usr/local/lib/python311.zip
