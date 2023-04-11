## go-pipeline

This repository demonstrates the pipeline pattern in Go, also contains other useful concurrency techniques/patterns.
The program is based on a real problem that Incentives Orchestration squad has to solve: we need to consume events from SQS, and batch them by size/time and send them to Perseus server.
In a nutshell, this problem can be solved by implementing a 3-stage approach. Each stage will provide the input stream for the next one.
- Stage 1: consuming SQS events with a dummy function, producing an event stream for stage 2 
- Stage 2: consuming event stream, batching SQS events by size/time, producing a batch stream for stage 3
- Stage 3: consuming batch stream, calling a function that pretends to send data to Perseus 

Inspiration for the approach: https://go.dev/blog/pipelines

## Notable patterns and techniques

This section details some patterns and techniques used in the demonstration.

### The generator patterns
When designing a multi-stage pipeline, the generator pattern could be extremely useful, by separating the outbound and inbound streams.
The generator takes in a stream of data, spinning a goroutine to pass the members of the straem to an outbound channel, which is then returned from the function.
```golang
func generator(nums []int) <-chan int {
    out := make(chan int)

    go func() {
        for _, n := range nums {
            out <- n
        }
        close(out)
    }()

    return out
}
```
Note that a rule of thumb here: a generator starts a new outbound channel, returns it, and is responsible for closing it. 
This is important for cancellation technique below.

### Cancellation
To synchronize cancellation between different goroutines, or different stages, use context.
Some Go contexts can be cancelled (by using `context.WithTimeout`, `context.WithDeadline` or `context.WithCancel`).

In this demonstration, we use `signal.NotifyContext` to handle termination signal, which calls `context.WithCancel` under the hood.
How does this actually work? When the application wants to terminate, the `stop()` function is called, which triggers the closing of the done channel of `ctx`.

In `receiveSQSMessages()`, the generator stops producing event once this cancellation signal is received by listening to `ctx.Done()`. 
```golang
case <-ctx.Done():
    return 
}
```

This in turns triggers `batchEvents()` to send the last batch, and close the batch stream
```golang
case e, ok := <-eventStream:
    if !ok {
        log.Println("batchEvents: sending the last batch when eventStream is closed")
        batchStream <- b
        return
    }
```

In the last stage, the for loop exits when `batchStream` is closed
```golang
go func() {
    for b := range batchStream {
        sendToPerseus(b)
    }

    close(done)
}()
```

### Confinement
Data race is a concurrency problem. 
Notice that in `batchEvents()`, we make a slice of events `b := make(batch, 0, 100)`, which is not a thread-safe data structure, but we never have to apply any locking technique for that because the usage of `b` is "confied" within the function.
When possible, confine the scope of not thread-safe data structure so that it won't be accessible by multiple goroutines

### Graceful shutdown