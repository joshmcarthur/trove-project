#!/usr/bin/env bash
set -e

# Default values
HEAD=false
DENO_VERSION="2.2.8"
TROVE_CLI_URL="https://raw.githubusercontent.com/joshmcarthur/trove-project/refs/heads/main/core/cli.ts"
TROVE_GIT="https://github.com/joshmcarthur/trove-project.git"
TROVE_BRANCH="${TROVE_BRANCH:-main}"
TROVE_INSTALL_DIR="$HOME/.trove"
TROVE_CLONE_DIR="$TROVE_INSTALL_DIR/git"
DENO_DIR="$TROVE_INSTALL_DIR/.deno"

# Parse bootstrap arguments while preserving remaining args for CLI
CLI_ARGS=()
while [[ "$#" -gt 0 ]]; do
    case $1 in
        --head)
            HEAD=true
            shift
            ;;
        *)
            CLI_ARGS+=("$1")
            shift
            ;;
    esac
done

# Restore CLI arguments
set -- "${CLI_ARGS[@]}"

# Detect OS
case "$(uname -s)" in
    Darwin*) OS="darwin" ;;
    Linux*) OS="linux" ;;
    MINGW*|MSYS*|CYGWIN*) OS="windows" ;;
    *) echo "Unsupported operating system"; exit 1 ;;
esac

# Function to check if specific deno version exists in our install
deno_version_exists() {
    [ -f "$DENO_DIR/bin/deno" ] && "$DENO_DIR/bin/deno" --version | grep -q "$DENO_VERSION"
}

# Create installation directories
mkdir -p "$DENO_DIR"

# Install specific Deno version if not present
if ! deno_version_exists; then
    echo "Installing Deno $DENO_VERSION to $DENO_DIR..."

    # Download Deno binary
    curl -fsSL https://deno.land/x/install/install.sh | CI=true DENO_INSTALL="$DENO_DIR" sh -s "v$DENO_VERSION"

    # Verify installation
    if ! deno_version_exists; then
        echo "Failed to install Deno v$DENO_VERSION. Please check your internet connection and try again."
        exit 1
    fi
fi

# Set up environment for Deno
export DENO_INSTALL="$DENO_DIR"
export PATH="$DENO_DIR/bin:$PATH"

echo "Deno $DENO_VERSION is ready!"

if [ "$HEAD" = true ]; then
    echo "Installing development version from Git..."

    # Create installation directory if it doesn't exist
    mkdir -p "$TROVE_INSTALL_DIR"

    # Clone or update repository
    if [ -d "$TROVE_CLONE_DIR/.git" ]; then
        echo "Updating existing repository..."
        cd "$TROVE_CLONE_DIR"
        git checkout "$TROVE_BRANCH"
        git pull origin "$TROVE_BRANCH"
    else
        echo "Cloning repository..."
        mkdir -p "$TROVE_CLONE_DIR"
        git clone "$TROVE_GIT" "$TROVE_CLONE_DIR" --branch "$TROVE_BRANCH"
        cd "$TROVE_CLONE_DIR"
    fi

    echo "Starting Trove from local installation..."
    "$DENO_DIR/bin/deno" run --allow-net --allow-read --allow-write --allow-env --allow-run "$TROVE_CLONE_DIR/core/cli.ts" "$@"
else
    echo "Starting Trove from stable release..."
    "$DENO_DIR/bin/deno" run --allow-net --allow-read --allow-write --allow-env --allow-run "$CLI_URL" "$@"
fi