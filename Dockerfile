# Support setting various labels on the final image
ARG COMMIT=""
ARG VERSION=""
ARG BUILDNUM=""

# Build go-ethereum-gnosis in a stock Go builder container
FROM golang:1.24-alpine AS builder

RUN apk add --no-cache gcc musl-dev linux-headers git

# Get dependencies - will also be cached if we won't change go.mod/go.sum
COPY go.mod /go-ethereum-gnosis/
COPY go.sum /go-ethereum-gnosis/
RUN cd /go-ethereum-gnosis && go mod download

ADD . /go-ethereum-gnosis
RUN cd /go-ethereum-gnosis && go run build/ci.go install -static ./cmd/geth

# Pull go-ethereum-gnosis into a second stage deploy alpine container
FROM alpine:latest

# Re-declare ARGs for the second stage since they don't carry over between stages
ARG COMMIT=""
ARG VERSION=""
ARG BUILDNUM=""

RUN apk add --no-cache ca-certificates
COPY --from=builder /go-ethereum-gnosis/build/bin/geth /usr/local/bin/

EXPOSE 8545 8546 30303 30303/udp
ENTRYPOINT ["geth"]

# Add some metadata labels to help programmatic image consumption
LABEL commit="$COMMIT" version="$VERSION" buildnum="$BUILDNUM"
