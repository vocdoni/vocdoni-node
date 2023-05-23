Return a filtered elections list, the available filters have to be sent on the request body. Available are: 
        
```json
{
    "organizationId": "hexString",
    "electionId": "hexString",
    "withResults": false,
    "status": "READY",
}
```

`electionId` can be partial. 

See [elections list](elections-list)