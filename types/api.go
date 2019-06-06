package types

/*MessageRequest holds a decoded request but does not decode the body*/
type MessageRequest struct {
	ID string `json:"id"`
	Request map [string]interface{} `json:"request"`
	Signature string `json:"signature"`
}

/* the following structs hold content decoded from File API JSON objects */


type FetchFileRequest struct {
	ID string `json:"id"`
	Request struct{
		Method string `json:"method"`
		URI string `json:"uri"`
		Timestamp uint `json:"timestamp"`
	} `json:"request"`
	Signature string `json:"signature"`
}

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

type PinListRequest struct {
	ID string `json:"id"`
	Request struct {
		Method string `json:"method"`
		Timestamp uint `json:"timestamp"`
	} `json:"request"`
	Signature string `json:"signature"`
}

type PinFileRequest struct {
	ID string `json:"id"`
	Request struct {
		Method string `json:"method"`
		URI string `json:"uri"`
		Timestamp uint `json:"timestamp"`
	} `json:"request"`
	Signature string `json:"signature"`
}

type UnpinFileRequest struct {
	ID string `json:"id"`
	Request struct {
		Method string `json:"method"`
		URI string `json:"uri"`
		Timestamp uint `json:"timestamp"`
	} `json:"request"`
	Signature string `json:"signature"`
}