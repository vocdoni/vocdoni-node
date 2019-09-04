package router

func submitEnvelope(request routerRequest, router *Router) {
	// request.structure.ProcessId
	// request.structured.Payload
	// router.vochain.method?
	// submitEnvelope
}

func getEnvelopeStatus(request routerRequest, router *Router) {
	// request.structured.ProcessId
	// request.structured.Nullifier
	// getEnvelopeStatus
}

func getEnvelope(request routerRequest, router *Router) {
	// request.structured.ProcessId
	// request.structured.Nullifier
	// getEnvelope
}

func getEnvelopeHeight(request routerRequest, router *Router) {
	// request.structured.ProcessId
	// getEnvelopeHeight
}

func getProcessList(request routerRequest, router *Router) {
	// request.structured.From
	// request.structured.ListSize
	// getProcessList
}

func getEnvelopeList(request routerRequest, router *Router) {
	// request.structured.ProcessId
	// request.structured.From
	// request.structured.ListSize
	// getEnvelopeList
}
