# Release & Docker Hub

## Version

- Source of truth: [`VERSION`](../VERSION) (semver without `v`, e.g. `0.1.0`)
- Git tags: `v0.1.0`, `v1.2.3`, …
- Injected at build via `-ldflags` into `internal/version`
- Runtime: `./server -version` or `GET /`

## Local build

```bash
make docker-build
# or with explicit version
make docker-build VERSION=v0.1.0
```

## GitHub Release workflow

Push a semver tag to trigger [`.github/workflows/release.yml`](../.github/workflows/release.yml):

```bash
git tag v0.1.0
git push origin v0.1.0
```

### Required repository secrets

| Secret | Description |
|--------|-------------|
| `DOCKERHUB_USERNAME` | Docker Hub user or org |
| `DOCKERHUB_TOKEN` | Docker Hub access token ([create](https://hub.docker.com/settings/security)) |

### Published image tags

For tag `v1.2.3`, pushes to `{DOCKERHUB_USERNAME}/lowcode-database`:

- `1.2.3`
- `1.2`
- `1`
- `latest`
- `v1.2.3`

Pull example:

```bash
docker pull YOUR_USER/lowcode-database:1.2.3
```

Optional: set repository variable `IMAGE_NAME` in workflow `env` to change image name (default `lowcode-database`).
