# LibreDash Docker Compose

This is the primary single-instance LibreDash deployment. It runs exactly one
application process with one named state volume and one configured environment.

```sh
cp deployment.env.example deployment.env
./libredashctl init --admin-email admin@example.com --domain dash.example.com
./libredashctl start
./libredashctl first-login
```

Set the released `LIBREDASH_IMAGE` digest before initialization. HTTPS is
enabled by default through the Caddy overlay. Use `--no-https` only when a
trusted external HTTPS proxy fronts the localhost-bound application port.

Run `./libredashctl help` for backup, restore, upgrade, and rollback commands.
