FROM golang:1.23-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /trove ./cmd/trove

FROM alpine:3.20
RUN adduser -D -u 1990 trove
USER trove
COPY --from=build /trove /usr/local/bin/trove
ENTRYPOINT ["trove"]
