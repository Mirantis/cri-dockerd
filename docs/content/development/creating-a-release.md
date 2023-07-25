---
weight: 3
---

These are the steps in order to create a new release for `cri-dockerd`. These steps will need to be done by a project maintainer.

1. Setup the repo for a new release
    1. Change the version found in `VERSION`, `cmd/version/version.go`, and `packaging/common.mk` to the new version
    2. Create a PR with these changes and merge them to master
    3. Build the release artifacts using `make release`
    4. Verify the artifacts in the `build/release` directory and make sure they look correct
2. A maintainer creates a new draft release in the project's [releases section](https://github.com/Mirantis/cri-dockerd/releases)
    1. The name should follow semantic convention prepended with a 'v'
    2. A tag with the same name should be created on the latest commit to master from the previous step
    3. Release notes should be generated using the previous tag and the new tag as the range
    4. Check the box to **Set as a pre-release**
    5. Upload the release artifacts from the previous step
    5. Save as a draft
3. The release can now go through a review process to look for any issues
4. Change the draft to a published release
5. Celebrate :cheers:
