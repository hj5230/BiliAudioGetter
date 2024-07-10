# syntax=docker/dockerfile:1

FROM golang:1.22.1
RUN --mount=type=cache,target=/var/cache/apt \
    apt-get update && apt-get install -y ffmpeg

WORKDIR /biliaudiogetter

COPY go.mod go.sum* ./

RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .

RUN --mount=type=cache,target=/root/.cache/go-build \
    go build -o BiliAudioGetter

CMD ["./BiliAudioGetter"]
