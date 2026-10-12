Creates an election on the Vochain. 

To use this endpoint, you will need to provide a signed transaction that has been encoded on the client side using the **Vocdoni SDK**. This transaction, referred to as txPayload, references the election metadata by its URI and hash.

The node does not store election metadata: the organization hosts the document at the URI set in the transaction, and the hash lets participants check it.

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