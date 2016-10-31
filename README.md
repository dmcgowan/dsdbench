# Docker Storage Driver Benchmarks and Tests

`dsdbench` runs benchmarks and tests for storage driver configurations to help
figure out how the configuration will perform and which known issues the
daemon may be affected by in this configuration.

## Usage

`dsdbench` makes use of the golang `testing` package. The tests may need to be
run as root in order to successfully mount. Use `DOCKER_GRAPHDRIVER` and
`DOCKER_GRAPHDRIVER_OPTIONS` environment variables to configure.

### Run tests
```
$ DOCKER_GRAPHDRIVER=overlay2 go test -v .
```

### Run benchmarks
```
$ DOCKER_GRAPHDRIVER=overlay2 go test -run=NONE -v -bench .
```
