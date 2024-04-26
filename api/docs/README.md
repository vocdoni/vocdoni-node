# Generating Vocdoni Node OpenAPI 3 Documentation

## Notes for the developers

This notes are a fast "how to" of how to document a new vocdoni node endpoint. 

- Before you can merge any new endpoint some tags have to be added to the endpoint:

```
//    @Tags           Accounts
//    @Accept         json
//    @Produce        json
```

This tags are needed to properly show the information on the developer portal. 
The `Accounts` tag refer to the namespace where the endpoint behalf, see 
[`api.go tag.name`](../api.go) to see available tags.

- Some other useful parameters to know are:

```
// For path params
//    @Param            address    path        string    true    "Account address"
// For body objects 
//    @Param                    transaction    body        object{txPayload=string,metadata=string}    true    "Transaction payload and metadata object encoded using base64 "
//    @Security                BasicAuth
// If bearer token is needed (for some census ops for example)
```

## Generate vocdoni-node OAS3 specification (tldr)


This guide explains how to generate the `vocdoni-node` specification in OpenApi 3 
(OAS 3) format.

### 1. Generate Swagger File From Code

We employ the [swaggo](https://github.com/swaggo/swag) library to generate a Swagger 2.0 YAML file from comments within the code. Execute the following commands to generate the Swagger YAML file:

```bash
# Install swaggo
go get github.com/swaggo/swag/cmd/swag
# Navigate into api/ folder and run the command to generate the Swagger YAML file:
go run github.com/swaggo/swag/cmd/swag init --parseDependency --parseDepth 1 --parseInternal --md docs/descriptions --overridesFile docs/.swaggo -g api.go -d ./,docs/models/models.go -o ./docs/build
```

The `docs/models/models.go` file contains placeholder Go code that is utilized for generating models, which are not automatically generated with the preceding command. For instance, this happens with the 'transaction by block height and index' call that returns a `oneof` transaction model from the Protobuf library at runtime.

Swagger 2.0 does not support these types of responses. Consequently, we need to convert it to OAS3 to use the `oneof` feature, exclusively available in OAS3. This capability serves our requirements. For further details, please [see the discusion here](https://github.com/vocdoni/interoperability/issues/70#issuecomment-1598424008).

### 2. Convert to OAS3

We employ the Node.js package `api-spec-converter` to convert the generated Swagger file to OAS3 format:

```bash
# Install dependencies
npm install -g api-spec-converter
# Declare the output file
OAS3_FILE="docs/build/oas3.yaml"
# Convert the Swagger file to OAS3 format
api-spec-converter --from=swagger_2 --to=openapi_3 --syntax=yaml docs/build/swagger.yaml > $OAS3_FILE
```

The above commands will generate the `OAS3_FILE` file, compliant with the OAS3 specification.

### 3. Execute the Fix Script

We have chosen to use [`yq`](https://github.com/mikefarah/yq), a command-line YAML processor in Go, to simplify operations. Using pure `sed` or `awk` proved challenging and less scalable. Here's how to use `yq`:

```bash
# Get latest version of yq
go get github.com/mikefarah/yq/v4@latest

# Define the model and output file
MODEL_FILE="docs/models/transactions.yaml"
SCHEMA_FILE="docs/build/vocdoni-api.yaml"

# Run the modification command
model_file=$MODEL_FILE go run github.com/mikefarah/yq/v4 '
.components.schemas."api.GenericTransactionWithInfo".properties.tx = load(strenv(model_file)).target
' $OAS3_FILE > $SCHEMA_FILE
```

In the command above, we replace the content from the `MODEL_FILE` with the property .`components.schemas."api.GenericTransactionWithInfo".properties.tx` in the generated `OAS3_FILE`.


The resulting `SCHEMA_FILE` will be used to describe the `vocdoni-node` API.

### Full script

Assuming all deps are installed, here a simple script to test locally this:


```bash
#!/bin/bash
OAS3_FILE="docs/build/oas3.yaml"
MODEL_FILE="docs/models/transactions.yaml"
SCHEMA_FILE="docs/build/vocdoni-api.yaml"

# Build the swagger 2.0 from code comments
go run github.com/swaggo/swag/cmd/swag init --parseDependency --parseDepth 1 --parseInternal --md docs/descriptions --overridesFile docs/.swaggo -g api.go -d ./,docs/models/models.go -o ./docs/build

echo "Convert the Swagger file to OAS3 format"
api-spec-converter --from=swagger_2 --to=openapi_3 --syntax=yaml docs/build/swagger.yaml > $OAS3_FILE

echo "Run models additions"
model_file=$MODEL_FILE go run github.com/mikefarah/yq/v4 '
.components.schemas."api.GenericTransactionWithInfo".properties.tx = load(strenv(model_file)).target' $OAS3_FILE > $SCHEMA_FILE
```

The resulting file will be `SCHEMA_FILE="docs/build/vocdoni-api.yaml"`.



