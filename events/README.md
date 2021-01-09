## Events

The events package provides an event handling mechanism to process the events of the vocdoni-node.

Events coming from different sources (Scrutinizer, Vochain, Ethereum ...) are collected by a dispatcher
and processed by a set of workers.

Each worker has an event queue, so events are serially processed. The queues can be persisted on disk
thus enabling the recovery of the not processed events if the vocdoni-node is halted for some reason.

### Dispatcher

The dispatcher holds a list of workers and references for each worker queue.
The dispatcher main functionality is:

- Check the incoming raw event
- Wrap the event into a shared and generic `Event` struct
- Select the queue based on the event type thus selecting the worker.
- Add the event to the file queue for processing.

### Workers

There is one worker per event source. One worker is assigned to one source.
One source can have many events and each event is handled by a specific callback.

A callback contains the logic for processing an event.

Each worker can handle the events sync or async, even if they are extracted from the queue one by one,
the actual logic for processing the event can be parallelized (e.g. ethereum events need to be processed in order
and one event cannot afect the vochain state before its predecessor is processed).
This mechanism for enabling parallelization is determined by the callback of the event:

- If the callback is surrounded by an anonymous go routine then the event is handled asyncronously
- If the callback is not sorrounded by an anonymous go routine then the event is handled syncronously

### Available sources

- [x] Scrutinizer
- [ ] Keykeeper
- [ ] Vochain
- [ ] Ethereum
- [ ] Census

### Flow diagram

![](https://github.com/vocdoni/design/blob/add-events/drawio/events-diagram.jpg)
