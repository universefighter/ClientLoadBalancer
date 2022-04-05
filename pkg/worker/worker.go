package worker

import (
    "fmt"
    "net/http"
    "time"
    "os"
    "io"
    "io/ioutil"
//    "sync"
    "example/clientLB/pkg/types"
)



// Worker represents the worker that executes the job
type Worker struct {
    WorkerPool  chan types.WorkerData
    JobChannel  chan types.Job
    quit        chan bool
    RetryCount  int
    MyClient    *http.Client
}

func NewWorker(workerPool chan types.WorkerData, retryCount int,
               myClient *http.Client) Worker {
    return Worker{
        WorkerPool: workerPool,
        JobChannel: make(chan types.Job),
        quit:       make(chan bool),
        RetryCount: retryCount,
        MyClient:   myClient}
}

// Start method starts the run loop for the worker, listening for a quit channel in
// case we need to stop it
func (w Worker) Start() {
    go func() {
        //defer wg.Done()
        ip := ""
        status := types.StatusInit
        var latency int64 = 0
        serviceDown := false
        for {
            // register the current worker into the worker queue.
            w.WorkerPool <- types.WorkerData {
                                Status: status,
                                JobChannel: w.JobChannel,
                                IP: ip,
                                Latency: latency,
                                ServiceDown: serviceDown}
            select {
                case job := <-w.JobChannel:
                    // we have received a work request.
                    latency, serviceDown = serviceRequest(job.IP, w.RetryCount, w.MyClient)
                    ip = job.IP
                    status = types.StatusRunning

                case <-w.quit:
                    // we have received a signal to stop
                    status = types.StatusStopped
                    return
            }
        }
    }()
}

// Stop signals the worker to stop listening for work requests.
func (w Worker) Stop() {
    go func() {
        w.quit <- true
    }()
}

func serviceRequest(ip string, retryCount int, myClient *http.Client) (int64, bool) {

    start_time := time.Now()

    serviceDown := true

    for i:=0; i<retryCount; i++ {
    
        url := "http://" + ip

        response, err := myClient.Get(url)
        if err != nil {
            if os.IsTimeout(err) {
                fmt.Println("serviceRequest timeout", ip)
                continue;
            } else {
                fmt.Println("serviceRequest error:", err)
                return 0, true
            }
        }

        io.Copy(ioutil.Discard, response.Body)
   
        response.Body.Close()

        if response.StatusCode >= 200 && response.StatusCode <= 500 {
            serviceDown = false
            break;
        }
    }

    dur_time := time.Now().Sub(start_time)
    duration := dur_time.Nanoseconds()

    return duration, serviceDown

}