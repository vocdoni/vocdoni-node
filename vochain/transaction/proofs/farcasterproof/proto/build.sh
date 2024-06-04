#!/bin/bash
# models extracted from https://github.com/farcasterxyz/hub-monorepo/blob/main/protobufs/schemas/message.proto
# and https://github.com/farcasterxyz/hub-monorepo/blob/main/protobufs/schemas/username_proof.proto
# Unified in a single file farcastermessage.proto
# Added the package header to make it compatible with Go bindings.
protoc --go_out=. --go_opt=paths=source_relative farcastermessage.proto
