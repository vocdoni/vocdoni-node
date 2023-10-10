# Docker compose deploy

## Standalone

To deploy a standalone node copy the `env.example` file as `env`, define the vars according to your settings, and start the node with:

```bash
RESTART=always docker compose up -d
```

This command will download the precompiled remote image of the vocdoni node.
For compiling your own image, you can run as previous step the following command:

```bash
docker compose build
```

## With Watchtower

If you want to add watchtower (for automatic remote image upgrade), use the relevant profile:

```bash
RESTART=always COMPOSE_PROFILES=watchtower docker compose up -d
```

Note that the RESTART variable can be set to change the behaviour across restarts. To ensure the node is started after a reboot, set it to `always` or `unless-stopped`
