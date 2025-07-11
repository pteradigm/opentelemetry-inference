# Release Process

This project uses [semantic-release](https://semantic-release.gitbook.io/) for automated versioning and releases.

## How It Works

1. Commits to `main` trigger the release workflow
2. Semantic-release analyzes commit messages to determine version bump:
   - `feat:` → minor version (1.x.0)
   - `fix:` → patch version (1.0.x)
   - `BREAKING CHANGE:` → major version (x.0.0)
3. Automatically:
   - Creates GitHub release with changelog
   - Builds and pushes Docker images to GHCR
   - Updates CHANGELOG.md
   - Creates git tag

## Setup Required

### Branch Protection Issue

The release workflow needs to push commits back to `main`, which conflicts with branch protection rules. Choose one solution:

### Option 1: Fine-Grained Personal Access Token (Recommended)

1. Create a fine-grained PAT:
   - Go to Settings → Developer settings → Personal access tokens → Fine-grained tokens
   - Repository access: Select this repository
   - Permissions needed:
     - Actions: Read
     - Contents: Write
     - Issues: Write
     - Metadata: Read
     - Pull requests: Write
     - **Administration: Write** (for branch protection bypass)

2. Add as repository secret:
   - Name: `RELEASE_TOKEN`
   - Value: Your PAT

### Option 2: Configure Branch Protection

Allow the GitHub Actions bot to bypass protection:
1. Go to Settings → Branches
2. Edit protection rule for `main`
3. Add `github-actions[bot]` to bypass list

### Option 3: GitHub App (Most Secure)

For production use:
1. Create a GitHub App with necessary permissions
2. Install on repository
3. Use app token in workflow

## Manual Release

If automated release fails:

```bash
# Ensure you have the latest main
git checkout main
git pull

# Create release locally
npm install -g semantic-release
export GITHUB_TOKEN=your_pat_here
semantic-release --no-ci
```

## Troubleshooting

### "Repository rule violations" Error
- Ensure RELEASE_TOKEN is set with correct permissions
- Or configure branch protection bypass

### No Release Created
- Check commit messages follow conventional format
- Ensure commits since last release contain releasable changes

### Docker Push Fails
- Verify GITHUB_TOKEN has packages:write permission
- Check GHCR login succeeds