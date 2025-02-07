FROM cgr.dev/chainguard/go AS builder
COPY . /app
WORKDIR /app 
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o main .

# NOTE laurenz-k alternatives: `scratch`, `cgr.dev/chainguard/glibc-dynamic`
FROM cgr.dev/chainguard/static
COPY --from=builder /app/main /usr/bin/

ENTRYPOINT ["/usr/bin/main"]
