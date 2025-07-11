#!/bin/bash

# Setup script for Git hooks

set -e

REPO_ROOT=$(git rev-parse --show-toplevel)
HOOKS_DIR="$REPO_ROOT/.githooks"

echo "Setting up Git hooks..."

# Configure git to use our hooks directory
git config core.hooksPath "$HOOKS_DIR"

echo "✓ Git hooks configured to use $HOOKS_DIR"
echo ""
echo "Pre-commit hook will:"
echo "  - Check Go formatting (gofmt)"
echo "  - Check import ordering (goimports)"
echo "  - Prevent large files (>5MB)"
echo ""
echo "To bypass hooks temporarily, use: git commit --no-verify"
echo "To disable hooks, run: git config --unset core.hooksPath"