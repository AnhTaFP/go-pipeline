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

### Cancellation

### Graceful shutdown