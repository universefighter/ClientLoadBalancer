package types


type Job struct {
    IP string
}

type WorkerData struct {
    Status          int
    JobChannel      chan Job
    IP              string
    Latency         int64 //The latency for the last request
    ServiceDown     bool  //Whether last request failed
}

const (
    StatusInit = iota
    StatusRunning
    StatusStopped
)