#!/bin/bash
docker image build -t vocdoni/go-dvote:latest -f dockerfile.buildsuit
docker push vocdoni/go-dvote:latest
