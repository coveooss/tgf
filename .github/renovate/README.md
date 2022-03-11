## Renovate preset configuration to update TGF versions

### What is [Renovate](https://docs.renovatebot.com/)?

It's a bot that helps updating dependencies in your repository. Out of the box it supports a lot of packaging types like mvn, npm or docker. They are called managers. 

It supports external configurations they call [preset](https://docs.renovatebot.com/config-presets/).

### What is this preset?

It's a custom dependency manager built with [regex](https://docs.renovatebot.com/modules/manager/regex/) that detects tgf.config files in your repo and updates it with an up to date version tagged from this current repo. 

### How to configure it on your repo

In your Renovate configuration, you have an array name [extends](https://docs.renovatebot.com/configuration-options/#extends). This array allows you to add a shareable configuration preset. Go to https://docs.renovatebot.com/config-presets/#shareable-config-presets to read more about the subject. 

In this example it shows you how you add the tgf-update preset
```json
  "extends": [
    "config:base",
    "github>coveo/tgf-images-coveo//renovate/preset/tgf"
  ],
```

### What kind of PRs should I expect?

You'll have a PR that update your tgf version to the latest version possible whatever the current version you are on. That means your PR build should contains a tgf plan/plan-all command to get any errors before it's packaged to the deployment pipeline.

