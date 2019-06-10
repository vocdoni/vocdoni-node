package types


/*MessageRequest holds a decoded request but does not decode the body*/
type MessageRequest struct {
	ID string `json:"id"`
	Request map [string]interface{} `json:"request"`
	Signature string `json:"signature"`
}

/* the following structs hold content decoded from File API JSON objects */

//FetchFileRequest holds a fetchFile request message
type FetchFileRequest struct {
	ID string `json:"id"`
	Request struct{
		Method string `json:"method"`
		URI string `json:"uri"`
		Timestamp uint `json:"timestamp"`
	} `json:"request"`
	Signature string `json:"signature"`
}

//AddFileRequest holds an addFile request message
type AddFileRequest struct {
	ID string `json:"id"`
	Request struct {
		Method string `json:"method"`
		Type string `json:"type"`
		Content string `json:"content"`
		Name string `json:"name"`
		Timestamp uint `json:"timestamp"`
	} `json:"request"`
	Signature string `json:"signature"`
}

//PinListRequest holds a pinList request message
type PinListRequest struct {
	ID string `json:"id"`
	Request struct {
		Method string `json:"method"`
		Timestamp uint `json:"timestamp"`
	} `json:"request"`
	Signature string `json:"signature"`
}

//PinFileRequest holds a pinFile request message
type PinFileRequest struct {
	ID string `json:"id"`
	Request struct {
		Method string `json:"method"`
		URI string `json:"uri"`
		Timestamp uint `json:"timestamp"`
	} `json:"request"`
	Signature string `json:"signature"`
}

//UnpinFileRequest holds an unpinFile request message
type UnpinFileRequest struct {
	ID string `json:"id"`
	Request struct {
		Method string `json:"method"`
		URI string `json:"uri"`
		Timestamp uint `json:"timestamp"`
	} `json:"request"`
	Signature string `json:"signature"`
}


//FetchResponse holds a fetchResponse response to be sent from the router
type FetchResponse struct {
	ID string `json:"id"`
	Response struct {
		Content string `json:"content"`
		Request string `json:"request"`
		Timestamp int64 `json:"timestamp"`
	} `json:"response"`
	Signature string `json:"signature"`
}

//AddResponse holds an addResponse response to be sent from the router
type AddResponse struct {
	ID string `json:"id"`
	Response struct {
		Request string `json:"request"`
		Timestamp int64 `json:"timestamp"`
		URI string `json:"uri"`
	} `json:"response"`
	Signature string `json:"signature"`
}

//ListPinsResponse holds a listPinsResponse reponse to be sent from the router
type ListPinsResponse struct {
	ID string `json:"id"`
	Response struct {
		Files []byte `json:"files"`
		Request string `json:"request"`
		Timestamp int64 `json:"timestamp"`
	} `json:"response"`
	Signature string `json:"signature"`
}

//BoolResponse holds a boolResponse response to be sent from the router
type BoolResponse struct {
	ID string `json:"id"`
	Response struct {
		OK bool `json:"ok"`
		Request string `json:"request"`
		Timestamp int64 `json:"timestamp"`
	} `json:"response"`
	Signature string `json:"signature"`
}

//FailBody holds a fail message to be sent from the router
type FailBody struct {
	ID string `json:"id"`
	Error struct {
		Request string `json:"request"`
		Message string `json:"message"`
	} `json:"error"`
}