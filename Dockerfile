FROM --platform="${BUILDPLATFORM:-linux/amd64}" golang:1.21.0 as builder

ARG TARGETPLATFORM
ARG BUILDPLATFORM
ARG TARGETOS
ARG TARGETARCH

ENV GOPRIVATE="github.com/stealthrocket"

WORKDIR /go/src/github.com/stealthrocket/timecraft
COPY . .

RUN GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -o build/timecraft 

FROM --platform="${TARGETPLATFORM:-linux/amd64}" debian:12

RUN apt-get update && apt-get upgrade -y

COPY --from=builder /go/src/github.com/stealthrocket/timecraft/build/timecraft /bin/timecraft

ENTRYPOINT ["/bin/timecraft"]
