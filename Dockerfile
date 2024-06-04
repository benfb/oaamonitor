ARG GO_VERSION=1
FROM golang:${GO_VERSION}-bookworm as builder

WORKDIR /usr/src/app
COPY go.mod go.sum ./
RUN go mod download && go mod verify
COPY . .
RUN go build -v -o /run-app .
RUN go build -v -o /run-refresher ./refresher

ENV SUPERCRONIC_URL=https://github.com/aptible/supercronic/releases/download/v0.2.29/supercronic-linux-amd64 \
    SUPERCRONIC=supercronic-linux-amd64 \
    SUPERCRONIC_SHA1SUM=cd48d45c4b10f3f0bfdd3a57d054cd05ac96812b

RUN curl -fsSLO "$SUPERCRONIC_URL" \
 && echo "${SUPERCRONIC_SHA1SUM}  ${SUPERCRONIC}" | sha1sum -c - \
 && chmod +x "$SUPERCRONIC" \
 && mv "$SUPERCRONIC" "/usr/local/bin/${SUPERCRONIC}" \
 && ln -s "/usr/local/bin/${SUPERCRONIC}" /usr/local/bin/supercronic

RUN curl -fsSLO "https://github.com/DarthSim/overmind/releases/download/v2.5.1/overmind-v2.5.1-linux-amd64.gz" \
    && gunzip overmind-v2.5.1-linux-amd64.gz \
    && chmod +x overmind-v2.5.1-linux-amd64 \
    && mv overmind-v2.5.1-linux-amd64 /usr/local/bin/overmind

FROM buildpack-deps:bookworm-curl

RUN apt-get update && apt-get install -y \
    tmux \
    && rm -rf /var/lib/apt/lists/*

COPY --from=builder /run-app /usr/local/bin/
COPY --from=builder /run-refresher /usr/local/bin/
COPY --from=builder /usr/local/bin/supercronic /usr/local/bin/
COPY --from=builder /usr/local/bin/overmind /usr/local/bin/

COPY . /app

# COPY data/oaamonitor.db /data/oaamonitor.db

CMD ["overmind", "start", "-f", "/app/Procfile"]
