FROM golang:1.20-alpine as build

RUN apk update && apk add make git gcc libc-dev curl

WORKDIR /go/src/github.com/stealthrocket/timecraft
COPY . .

ARG version
ARG tags
ARG gocache

RUN go build -o timecraft 

# The last layer of the build, which determines what the final docker image
# will contain.
FROM alpine

RUN apk update && apk add ca-certificates curl bind-tools tree

ENV TIMECRAFT_BIND=:8080

EXPOSE 8080/tcp

COPY --from=build /go/src/github.com/stealthrocket/timecraft/timecraft /usr/bin/timecraft

ENTRYPOINT ["timecraft"]
