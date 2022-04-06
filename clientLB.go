package main

import (
    "fmt"
    "runtime"
    "strconv"
    "net/http"
    "time"
    "os"
    "example/clientLB/pkg/scheduler"
    //"example/clientLB/pkg/worker"
)

var myClient *http.Client

const TotalRequests int = 100

const RefreshIPTime int = 3000 //ms

var DefaultMaxWorkers int = runtime.NumCPU() * 2

const RetryCount int = 2


func main() {

    if len(os.Args) != 2 {
        fmt.Println("Usage: clientLB url")
        return
    }

    url := os.Args[1]

    t := http.DefaultTransport.(*http.Transport).Clone()
    t.MaxIdleConns = 100
    t.MaxConnsPerHost = 100
    t.MaxIdleConnsPerHost = 100
    myClient = &http.Client{
                   Transport: t,
                   Timeout:   2 * time.Second,}

    //worker.ServiceRequest("10.2.1.3",2, myClient)
    //return

    MaxWorkerStr := os.Getenv("MAX_WORKERS")

    var MaxWorkers = DefaultMaxWorkers

    if MaxWorkerStr != "" {

        MaxWorkerInt, err := strconv.Atoi(MaxWorkerStr)

        if err == nil {
            if MaxWorkerInt > 0 && MaxWorkerInt <= TotalRequests {
                MaxWorkers = MaxWorkerInt
            } else if MaxWorkerInt > TotalRequests {
                MaxWorkers = TotalRequests
            }
        }
    }
    fmt.Println("MAX Workers:", MaxWorkers)

    dispatcher := scheduler.NewDispatcher(MaxWorkers, url, TotalRequests, RetryCount, myClient, RefreshIPTime)
    if dispatcher != nil {
        dispatcher.Run()
    }
}