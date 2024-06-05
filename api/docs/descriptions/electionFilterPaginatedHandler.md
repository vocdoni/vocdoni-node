Returns a filtered list of elections. The filters have to be sent on the request body. The valid filters are: 
        
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