FROM --platform="${BUILDPLATFORM:-linux/amd64}" golang:1.20.5 as builder

ENV GOPRIVATE="github.com/stealthrocket"

WORKDIR /go/src/github.com/stealthrocket/timecraft
COPY . .

RUN go build -o build/timecraft 

#TODO: switch to distroless/debian:12 once released
FROM --platform="${BUILDPLATFORM:-linux/amd64}" debian:12

RUN apt-get update && apt-get upgrade -y

COPY --from=builder /go/src/github.com/stealthrocket/timecraft/build/timecraft /bin/timecraft

ENTRYPOINT ["/bin/timecraft"]
