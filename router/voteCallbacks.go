package router

func submitEnvelope(request routerRequest, router *Router) {

	//txbytes := []byte(vtest.HARDCODED_NEW_VOTE_TX)
	//req := abci.RequestDeliverTx{Tx: txbytes}
	//log.Infof("%+v", router.vochain.DeliverTx(req))
	/*
			var errMsg string
			log.Info("calling submit envelope")
			args := fmt.Sprintf(`{
				"method": voteTx,
				"args": {
					processId: %s,
					nullifier:
				}
			}`)
			envelopeData := vtypes.Tx {
				Method: request.method,
				Args: request.structured.,
			}
			//txbytes := []byte(vtest.HARDCODED_NEW_PROCESS_TX)
			//txbytes := []byte(vtest.HARDCODED_NEW_PROCESS_TX)

			//req := abci.RequestDeliverTx{Tx: txbytes}
			//log.Infof("%+v", router.vochain.DeliverTx(req))
			//go log.Infof("%+v", router.vochain.Commit())
			//req2 := abci.RequestDeliverTx{Tx: txbytes}
			//time.Sleep(10 * time.Second)
			//time.Sleep(5 * time.Second)
			//go vlog.Infof("%+v", app.DeliverTx(req2))

		/*
			{
				"id": "req-2345679",
				"request": {
				  "method": "submitEnvelope",
				  "processId": "hexString",
				  "payload": "base64-data",
				  "timestamp": 1556110671
				},
				"signature": "hexString"
			  }

	*/

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
