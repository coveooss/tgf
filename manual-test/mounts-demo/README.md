# Mounts Demo

Build `tgf` from the repo root:

```bash
go build -o ./tgf .
```

Check the generated docker mount from the demo app folder:

```bash
cd manual-test/mounts-demo/project/app
../../../../tgf -D --no-interactive -- true
```

You should see a bind mount like:

```text
-v /.../manual-test/mounts-demo/modules:/var/tgf/modules
```

If your configured image has Terraform available, you can also verify the mounted module resolves:

```bash
cd manual-test/mounts-demo/project/app
../../../../tgf --no-interactive --entrypoint terraform -- init
```
