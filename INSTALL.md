# Installing wiff

## Homebrew (macOS/Linux)

```
brew install h0rv/tap/wiff
```

## Go install (requires Go 1.24+)

```
go install github.com/h0rv/wiff@latest
```

## Shell script (macOS/Linux)

Downloads the latest release binary for your platform:

```
curl -fsSL https://raw.githubusercontent.com/h0rv/wiff/main/install.sh | sh
```

To install to a custom directory:

```
INSTALL_DIR=~/.local/bin curl -fsSL https://raw.githubusercontent.com/h0rv/wiff/main/install.sh | sh
```

## Pre-built binaries

Download from [GitHub Releases](https://github.com/h0rv/wiff/releases).

Available for:
- macOS (arm64, amd64)
- Linux (arm64, amd64)
- Windows (arm64, amd64)

```
# Example: macOS arm64
tar xzf wiff_darwin_arm64.tar.gz
sudo mv wiff /usr/local/bin/

# Example: Linux amd64
tar xzf wiff_linux_amd64.tar.gz
sudo mv wiff /usr/local/bin/
```

## Build from source

```
git clone https://github.com/h0rv/wiff.git
cd wiff
go build -o wiff .
sudo mv wiff /usr/local/bin/
```

## Verify installation

```
wiff --version
```
