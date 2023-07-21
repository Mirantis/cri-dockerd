These are the steps in order to create a new release for `cri-dockerd`. These steps will need to be done by a project maintainer.

1. Setup the repo for a new release
  a. Change the version found in `VERSION`, `cmd/version/version.go`, and `packaging/common.mk` to the new version
  b. Create a PR with these changes and merge them to master
  c. Build the release artifacts using `make release`
  d. Verify the artifacts in the `build/release` directory and make sure they look correct
2. A maintainer creates a new draft release in the project's [releases section](https://github.com/Mirantis/cri-dockerd/releases)
  a. The name should follow semantic convention prepended with a 'v'
  b. A tag with the same name should be created on the latest commit to master from the previous step
  c. Release notes should be generated using the previous tag and the new tag as the range
  d. Check the box to **Set as a pre-release**
  e. Upload the release artifacts from the previous step
  e. Save as a draft
3. The release can now go through a review process to look for any issues
4. Change the draft to a published release
5. Celebrate :beers:
