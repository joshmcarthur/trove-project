FROM golang:1.26-alpine AS build
WORKDIR /src
ARG VERSION=dev
ARG COMMIT=none
ARG DATE=unknown
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build \
  -ldflags "-s -w -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.date=${DATE}" \
  -o /trove ./cmd/trove

FROM alpine:3.24
RUN adduser -D -u 1990 trove
USER trove
COPY --from=build /trove /usr/local/bin/trove
ENTRYPOINT ["trove"]
