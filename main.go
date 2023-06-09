package main

import (
	"context"
	"log"
	"math/rand"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/google/uuid"
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
				select {
				case eventStream <- e:
				case <-ctx.Done():
					return
				}

			case <-ctx.Done():
				return
			}
		}
	}()

	return eventStream
}

// batchEvents batches up to 5 events, or every 5 seconds, then sends the batch to a batch stream,
// which is returned from the function.
func batchEvents(ctx context.Context, eventStream <-chan event) <-chan batch {
	batchStream := make(chan batch)
	b := make(batch, 0, 5)

	go func() {
		defer close(batchStream)

		for {
			select {
			case e, ok := <-eventStream:
				if !ok {
					log.Println("batchEvents: sending the last batch when eventStream is closed")
					select {
					case batchStream <- b:
					case <-ctx.Done():
					}

					return
				}

				log.Println("batchEvents: batching events")
				b = append(b, e)

				if len(b) == 5 {
					log.Println("batchEvents: sending batched events to batchStream")
					select {
					case batchStream <- b:
					case <-ctx.Done():
						return
					}
					b = make(batch, 0, 5)
				}
			case <-time.After(5 * time.Second):
				log.Println("batchEvents: sending batched events to batchStream because 5 seconds has passed")
				select {
				case batchStream <- b:
				case <-ctx.Done():
					return
				}
				b = make(batch, 0, 5)
			case <-ctx.Done():
				return
			}
		}
	}()

	return batchStream
}

// sendToPerseus pretends to send data to Perseus with a latency of 50 Milliseconds
func sendToPerseus(ctx context.Context, b batch) {
	select {
	case <-ctx.Done():
	default:
		id := uuid.NewString()

		log.Printf("sending a batch of %d events to perseus\n, batch id: %s", len(b), id)
		time.Sleep(5 * time.Second)
		log.Printf("finished batch id %s\n", id)
	}
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)

	// stage 1
	eventStream := receiveSQSMessages(ctx)

	// stage 2
	batchStream := batchEvents(ctx, eventStream)

	// stage 3
	done := make(chan struct{})
	go func() {
		var wg sync.WaitGroup
		wg.Add(10)

		for i := 0; i < 10; i++ {
			go func() {
				for b := range batchStream {
					sendToPerseus(ctx, b)
				}

				wg.Done()
			}()
		}

		wg.Wait()
		close(done)
	}()

	select {
	case <-time.After(20 * time.Second):
		stop()
		log.Println("shutting down the application after 20 seconds, waiting for the last batch to be sent for 5 seconds")
		select {
		case <-done:
			log.Println("shut down application because no more batches to send")
		case <-time.After(5 * time.Second):
			log.Println("shut down application because 5 seconds has passed")
		}
	case <-ctx.Done():
		log.Println("shutting down the application because of receiving termination signal, waiting for the last batch to be sent for 5 seconds")
		select {
		case <-done:
			log.Println("shut down application because no more batches to send")
		case <-time.After(5 * time.Second):
			log.Println("shut down application because 5 seconds has passed")
		}
	}
}
