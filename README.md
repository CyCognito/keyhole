# Keyhole - Survey Your Mongo Land

Keyhole is a tool to explore MongoDB deployments. Instructions are available at

[![Survey Your Mongo Land with Keyhole and Maobi](https://img.youtube.com/vi/kObLsYJAruI/0.jpg)](https://youtu.be/kObLsYJAruI?si=Tv2Qbd2vHATt0WH1).

For MongoDB JSON logs parsing, use the improved [Hatchet](https://github.com/simagix/hatchet) project.

## Running with Docker

### Building the Docker Image

Build the Docker image from the Dockerfile:

```bash
docker build -t keyhole .
```

### Basic Usage

Check the version:

```bash
docker run --rm keyhole --version
```

### Connecting to MongoDB

Run keyhole with a MongoDB connection string:

```bash
docker run --rm keyhole <connection_string>
```

For example, to get cluster information:

```bash
docker run --rm keyhole -allinfo "mongodb://localhost:27017"
```

### Working with Output Files

If you need to save output files (e.g., HTML reports, BSON files), mount a volume:

```bash
docker run --rm \
  -v $(pwd)/out:/home/simagix/out \
  -v $(pwd)/html:/home/simagix/html \
  keyhole -allinfo "mongodb://localhost:27017" -html
```

This will save output files to the `out` and `html` directories in your current working directory.

### Analyzing Log Files

To analyze MongoDB log files, mount the directory containing your logs:

```bash
docker run --rm \
  -v $(pwd)/logs:/home/simagix/logs \
  keyhole -loginfo logs/mongod.log
```

### Network Considerations

If your MongoDB instance is running on the host machine, use `--network host` (Linux) or `host.docker.internal` (macOS/Windows):

**Linux:**
```bash
docker run --rm --network host keyhole -allinfo "mongodb://localhost:27017"
```

**macOS/Windows:**
```bash
docker run --rm keyhole -allinfo "mongodb://host.docker.internal:27017"
```

### Building from Local Source

If you want to build from the local repository instead of the upstream, modify the Dockerfile to copy local files instead of cloning from GitHub.

## Running from Source

### Prerequisites

- Go 1.18 or later installed on your system
- Git (for version information)

### Building the Binary

Build keyhole using the provided build script:

```bash
./build.sh
```

This will create a binary at `dist/keyhole`. The script will also verify the build by running `keyhole --version`.

Alternatively, you can build it manually:

```bash
go build -o dist/keyhole main/keyhole.go
```

### Running the Binary

After building, you can run keyhole directly:

```bash
./dist/keyhole -allinfo "mongodb://localhost:27017"
```

### Running with HTML Report Generation

To generate an HTML report with cluster information, use the `-html` flag:

```bash
./dist/keyhole -allinfo "mongodb://localhost:27017" -html
```

The HTML report will be saved in the `html` directory (created automatically if it doesn't exist). The file will be named based on your MongoDB connection string, for example: `html/your-cluster-stats.html`.

You can also generate HTML reports for other operations:

```bash
# Generate HTML report for cluster stats
./dist/keyhole -allinfo "mongodb://localhost:27017" -html

# Analyze logs and generate reports (if supported)
./dist/keyhole -loginfo mongod.log
```

### Running Without Building

You can also run keyhole directly from source without building a binary:

```bash
go run main/keyhole.go -allinfo "mongodb://localhost:27017" -html
```

This is useful for development and testing, but building the binary is recommended for production use.

### Output Directories

When running locally, keyhole creates the following directories for output:
- `./html/` - HTML reports and visualization files
- `./out/` - BSON data files and other outputs

These directories are created automatically in your current working directory.

## License

[Apache-2.0 License](LICENSE)

## Disclaimer

This software is not supported by MongoDB, Inc. under any of their commercial support subscriptions or otherwise. Any usage of keyhole is at your own risk.

## Changes
### v1.3.x
- `-allinfo` supports high number of collections
- `-loginfo` includes raw logs

### v1.2.1
- Supports ReadPreferenceTagSets

### v1.2
- Prints connected client information from all `mongod` processes
- Prints client information from a log file
- Compares two clusters using internal metadata
- Performs deep comparison of two clusters
