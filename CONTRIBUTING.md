# Contributing

## Release procedure

1. Find the latest tag
1. Look at changes between now and the current tag
1. Create a tag with the format `vX.Y.Z`

    You might be tempted to create a release through GitHub to create the tag.
    Don't do that. We have a GitHub workflow in place that will create a release
    from tags that get pushed. Create your tag with `git tag`.

1. Push that tag
1. Wait for the [Release on Tag](https://github.com/coveooss/tgf/actions/workflows/tag.yml) to build your release.
1. Find your release and make sure things are as you expect them to be
