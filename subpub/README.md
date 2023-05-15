# SubPub

SubPub is a simplified PubSub protocol using libp2p

* Creates a libp2p Host, with a PubSub service attached that uses the GossipSub router.
* method SendBroadcast() uses this PubSub service for routing broadcast messages to peers.
* method SendUnicast() sends a unicast packet directly to a single peer.
* The topic used for p2p discovery is determined from the groupKey (hash)
* Incoming messages (both unicasts and broadcasts) are passed to the `messages` channel indicated in Start()
