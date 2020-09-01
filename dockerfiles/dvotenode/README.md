# Docker compose deploy

## Standalone

To deploy a standalone dvotenode copy the `env.example` file as `env`, define the vars according to your settings, and start the node with:

```bash
RESTART=always docker-compose up -d
```

## With Web3 or Watchtower

If you want to add an extra Web3 side container, or a watchtower, provide all compose files involved:

```bash
RESTART=always docker-compose -f docker-compose.yml -f docker-compose.web3.yml -f docker-compose.watchtower.yml up -d
```

Similarly, you need to provide all compose files to tear it down:

```bash
RESTART=always docker-compose -f docker-compose.yml -f docker-compose.web3.yml -f docker-compose.watchtower.yml down
```

Note that the RESTART variable can be set to change the behaviour across restarts. To ensure the node is started after a reboot, set it to `always` or `unless-stopped`
