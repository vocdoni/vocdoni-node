# Generate a docker-compose.yml for a custom-sized testnet

you need to have [ansible](https://ansible.com) installed, then you can run:

```sh
ansible-playbook generate_testnet.yml
```

you can also pass parameters like this:

```sh
ansible-playbook generate_testnet.yml -e gateways=3 -e seeds=2 -e oracles=2 -e miners=10
```

this will generate various files, mainly a `docker-compose.yml`, `genesis.json` and some `env` files.

afterwards, you can run the usual test script but using this freshly generated custom environment:

```sh
../start_test.sh
```
