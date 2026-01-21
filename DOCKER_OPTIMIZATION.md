# Docker Build Optimization

## Changes Made

### 1. Dockerfile Optimizations

#### Frontend Build
- **npm ci instead of npm install**: Faster, reproducible builds using package-lock.json
- **--prefer-offline --no-audit**: Skip unnecessary network requests and audits
- **Better layer caching**: Copy package.json first, then source files

#### Backend Build
- **Go 1.23 instead of 1.24**: Use stable released version
- **-trimpath flag**: Removes file system paths from binary (smaller size, better reproducibility)
- **Optimized ldflags**: `-s -w` removes debug info and symbol table

#### Runtime Image
- **Alpine 3.19**: Specific version for better reproducibility
- **Combined RUN commands**: Fewer layers = smaller image

### 2. GitHub Actions Optimizations

#### Cache Strategy
- **GitHub Actions cache (GHA)**: Caches build layers between runs
- **Registry cache**: Additional cache from Docker Hub (optional)
- **mode=max**: Caches all layers, not just final image

#### Build Optimizations
- **provenance: false**: Skips attestation generation (faster builds)
- **sbom: false**: Skips SBOM generation (faster builds)

### 3. .dockerignore Improvements
- Excludes test files, development files, and build artifacts
- Reduces build context size significantly
- Faster context transfer to Docker daemon

## Expected Performance Improvements

### First Build (Cold Cache)
- **Before**: ~6 minutes
- **After**: ~4-5 minutes
- **Improvement**: ~20-30% faster

### Subsequent Builds (Warm Cache)
- **Before**: ~6 minutes
- **After**: ~1-2 minutes (if only code changes)
- **Improvement**: ~70-80% faster

### Cache Hit Scenarios
- **No changes**: ~30 seconds (pure cache)
- **Frontend only**: ~1-2 minutes
- **Backend only**: ~1-2 minutes
- **Both changed**: ~3-4 minutes

## Additional Optimization Tips

### For Even Faster Builds
1. **Use self-hosted runners**: Persistent cache, faster network
2. **Build single platform first**: Test on amd64 only, then multi-platform on release
3. **Separate workflows**: Different workflows for PR (fast) vs release (complete)

### Example: Fast PR Workflow
```yaml
# For PRs: build only amd64, no push
platforms: linux/amd64
push: false
```

### Example: Release Workflow
```yaml
# For releases: build all platforms, push
platforms: linux/amd64,linux/arm64
push: true
```

## Monitoring Build Times

Check your GitHub Actions runs to see the improvement:
- Go to: Actions → Build and Publish Docker Image
- Compare build times before and after optimization
- Look for "Build and push Docker image" step duration

## Cache Management

### Clear Cache (if needed)
```bash
# In GitHub Actions settings
Settings → Actions → Caches → Delete specific cache
```

### Local Testing
```bash
# Build with cache
docker buildx build --cache-from type=local,src=/tmp/buildx-cache \
                    --cache-to type=local,dest=/tmp/buildx-cache \
                    -t nginx-log-viewer:test .

# Build without cache (to test cold build)
docker buildx build --no-cache -t nginx-log-viewer:test .
```
