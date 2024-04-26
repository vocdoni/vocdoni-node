Creates an election on the Vochain. 

To use this endpoint, you will need to provide a signed transaction that has been encoded on the client side using the **Vocdoni SDK**. This transaction, referred to as txPayload, must include the IPFS CID-formatted hash of the metadata for the election.

The metadata for the election is optional and is provided as a base64-encoded JSON object. This object should follow the Entity metadata specification and includes information about the election, such as the list of candidates and the election's description. This metadata is stored within IPFS so that participants can access it.

The API endpoint will verify that the hash in the txPayload transaction matches the uploaded metadata. If these do not match, the API will return an error.

Example of election metadata object:

```json
{
    "version": "1.0",
    "title": {"default": "Best pasta!", "en": "Best pasta!", "es": "La mejor pasta!"},
    "description": {"default": "Decide what is the best pasta", "en": "Decide what is the best pasta", "es": "Decide cual es la mejor pasta"},
    // Following fields are optional
    "media": {
    "header": "url to an image"
    "streamUri": "url to a stream resource"
    },
    "questions": [
    {
        "choices": [ 
        { 
            "title": {"default": "Macarroni", "en": "Macarroni", "es": "Macarrones"},
            "value": 0
        } 
        { 
            "title": {"default": "Spaghetti", "en": "Spaghetti", "es": "Espaguetis"},
            "value": 1
        } 
        ], 
        "description": {"default": "Choice one of theme", "en": "Choice one of theme", "es": "Elije una de ellas"},
        "title": {"default": "Macarroni or Spaghetti", "en": "Macarroni or Spaghetti", "es": "Macarrones o Espaguetis"}
    }
    ]
}
```

[Read more about process creation](https://docs.vocdoni.io/architecture/process-overview.html#process-creation)