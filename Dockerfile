FROM golang:1.23-alpine AS builder
RUN apk update && apk add git bash && rm -rf /var/cache/apk/*
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# Initialize git repo if .git is missing (needed for build.sh version detection)
RUN if [ ! -d .git ]; then \
      git init && \
      git config user.email "build@docker.local" && \
      git config user.name "Docker Build" && \
      git add -A && \
      git commit -m "Docker build"; \
    fi
# Ensure go.mod and go.sum are up to date
RUN go mod tidy
# Download dependencies for all platforms to ensure go.sum is complete
RUN GOOS=darwin GOARCH=amd64 go mod download all
RUN GOOS=linux GOARCH=amd64 go mod download all  
RUN GOOS=windows GOARCH=amd64 go mod download all
RUN go mod tidy && go mod verify
RUN ./build.sh cross-platform
FROM alpine
LABEL maintainer="Ken Chen <ken.chen@simagix.com>"
RUN addgroup -S simagix && adduser -S simagix -G simagix
USER simagix
WORKDIR /dist
COPY --from=builder /build/dist/keyhole-* /dist/
WORKDIR /home/simagix
COPY --from=builder /build/dist/keyhole-linux-x64 /keyhole
ENTRYPOINT ["/keyhole"]
CMD ["--version"]