package main

import (
	"context"
	"log"
	"math/rand"
	"time"
)

type event struct{}

type batch []event

// receiveSQSMessages pretends to receive sqs message every 1-20 milliseconds,
// then sends that event to an event stream, which is then returned.
func receiveSQSMessages(ctx context.Context) <-chan event {
	eventStream := make(chan event)
	go func() {
		defer close(eventStream)
		for {
			interval := rand.Intn(20)
			if interval == 0 {
				interval = 1
			}

			select {
			case <-time.After(time.Duration(interval) * time.Millisecond):
				log.Println("receiveSQSMessages: sending event to eventStream")
				e := event{}
				eventStream <- e
			case <-ctx.Done():
				return
			}
		}
	}()

	return eventStream
}

// batchEvents batches up to 100 events, or every 5 seconds, then sends the batch to a batch stream,
// which is returned from the function.
func batchEvents(ctx context.Context, eventStream <-chan event) <-chan batch {
	batchStream := make(chan batch)
	b := make(batch, 0, 100)

	go func() {
		defer close(batchStream)

		for {
			select {
			case e, ok := <-eventStream:
				if ok {
					log.Println("batchEvents: batching events")
					b = append(b, e)

					if len(b) == 100 {
						log.Println("batchEvents: sending batched events to batchStream")
						batchStream <- b
						b = b[:0]
					}
				}
			case <-time.After(5 * time.Second):
				log.Println("batchEvents: sending batched events to batchStream because 5 seconds has passed")
				batchStream <- b
				b = b[:0]
			case <-ctx.Done():
				return
			}
		}
	}()

	return batchStream
}

// sendToPerseus pretends to send data to Perseus with a latency of 50 Milliseconds
func sendToPerseus(b batch) {
	log.Printf("sending a batch of %d events to perseus\n", len(b))
	time.Sleep(50 * time.Millisecond)
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())

	eventStream := receiveSQSMessages(ctx)
	batchStream := batchEvents(ctx, eventStream)

	done := make(chan struct{})

	go func() {
		for b := range batchStream {
			sendToPerseus(b)
		}

		close(done)
	}()

	select {
	case <-time.After(20 * time.Second):
		cancel()
		log.Println("shutting down the application after 1 minute, waiting for the last batch to be sent for 5 seconds")
		select {
		case <-done:
			log.Println("shut down application because no more batches to send")
		case <-time.After(5 * time.Second):
			log.Println("shut down application because 5 seconds has passed")
		}
	}
}
