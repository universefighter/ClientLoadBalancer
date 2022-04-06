package scheduler

import (
    "fmt"
    "os/exec"
    "strings"
    "strconv"
    "time"
//    "sync"
    "net/http"
    "example/clientLB/pkg/types"
    "example/clientLB/pkg/worker"
)

//var wg sync.WaitGroup


type IPData struct {
    RequestCount   int
    DesiredWorders int
    ActualWorkers  int
    MinLatency     uint64
    MaxLatency     uint64
    AvgLatency     uint64
    TotalLatency   uint64
}

type Dispatcher struct {
    // A pool of workers channels that are registered with the dispatcher
    WorkerPool          chan types.WorkerData
    URL                 string
    //IPs behind the DNS of URL
    IPs                 []string
    //store the IP related IPData
    IPsMap              map[string]*IPData
    //store the worker num the IP still needed
    NeedAssignIPMap     map[string]int
    maxWorkers          int
    RetryCount          int
    TotalRequests       int
    RefreshIPTime       int
    MyClient            *http.Client
}

func initIpsMap(ips []string, ipsMap map[string]*IPData, maxWorkers int) {
    workersPerIp := maxWorkers/len(ips)
    for _, v := range ips {
        ipsMap[v] = &IPData {
                        RequestCount: 0,
                        DesiredWorders: workersPerIp,
                        ActualWorkers: workersPerIp,
                        MinLatency: ^uint64(0),
                        MaxLatency: 0,
                        AvgLatency: 0,
                        TotalLatency: 0}
    }

    if remain :=maxWorkers - workersPerIp*len(ips); remain > 0 {
        ipsMap[ips[0]].DesiredWorders += remain
        ipsMap[ips[0]].ActualWorkers += remain
    }

}

func NewDispatcher(maxWorkers int, url string,
                   totalRequests int, retryCount int,
                   myClient *http.Client, refreshIPTime int) *Dispatcher {

    ips, err := getIps(url)
    if err != nil || len(ips) == 0 {
        return nil
    }

    pool := make(chan types.WorkerData, maxWorkers)
    ipsMap := make(map[string]*IPData, len(ips))
    needAssignIPMap := make(map[string]int)
    initIpsMap(ips, ipsMap, maxWorkers)
    return &Dispatcher {
                WorkerPool: pool,
                URL: url,
                IPs: ips,
                IPsMap: ipsMap,
                NeedAssignIPMap: needAssignIPMap,
                maxWorkers: maxWorkers,
                RetryCount: retryCount,
                TotalRequests: totalRequests,
                RefreshIPTime: refreshIPTime,
                MyClient: myClient}
}

func (d *Dispatcher) Run() {

//    wg.Add(d.maxWorkers)

    start_time := time.Now()

    workers := make([]worker.Worker, d.maxWorkers, d.maxWorkers)
    for i := 0; i < d.maxWorkers; i++ {
        workers[i] = worker.NewWorker(d.WorkerPool, d.RetryCount, d.MyClient)
        workers[i].Start()
    }

    d.dispatch()

    for i := 0; i < d.maxWorkers; i++ {
        workers[i].Stop()
    }

//    wg.Wait()

    dur_time := time.Now().Sub(start_time)
    fmt.Println("Total duration:", dur_time.Nanoseconds()/(1000*1000), "ms")

    for key, v := range d.IPsMap {
        if v.RequestCount != 0 {
            fmt.Println("IP:", key, "RequestCount:", v.RequestCount, "MinLatency:",
                     v.MinLatency/(1000*1000), "ms MaxLatency:", v.MaxLatency/(1000*1000),
                     "ms AvgLatency:",(v.TotalLatency/(uint64)(v.RequestCount))/(1000*1000),
                     "ms desiredWorker:", v.DesiredWorders, "actualWorker:", v.ActualWorkers)
        } else {
            fmt.Println("IP:", key, "RequestCount:", v.RequestCount, "MinLatency: N/A",
                        "MaxLatency: N/A", "AvgLatency: N/A",
                        "desiredWorker:", v.DesiredWorders, "actualWorker:", v.ActualWorkers)
        }
    }
}

func (d *Dispatcher) dispatch() {
    ipLen := len(d.IPs)
    refreshIPTimer := time.After(time.Duration(d.RefreshIPTime) * time.Millisecond)
    waited := d.maxWorkers
    existError := false
    for i:=0; i<d.TotalRequests; {
        if existError {break}
        select {
            //get IPs from DNS again every 3 seconds
            case <- refreshIPTimer:
                fmt.Println("refresh IP")
                ips, err := getIps(d.URL)
                if err != nil || len(ips) == 0 {
                    existError = true
                    break;
                }

                oldIPs := d.IPs
                d.IPs = ips
                if (!isIPSame(d.IPs, d.IPsMap)){
                    d.reAssignIPWorkers(oldIPs)
                }
                refreshIPTimer = time.After(time.Duration(d.RefreshIPTime) * time.Millisecond)
            //worker available
            case workerData := <-d.WorkerPool:
                if workerData.Status == types.StatusInit {
                    //Init state, Just round robin assign each IP to worker
                    workerData.JobChannel <- types.Job{IP: d.IPs[i%ipLen]}
                } else if workerData.Status == types.StatusRunning {
                    //running state and service request succussful
                    if workerData.ServiceDown != true {
                        ip := workerData.IP
                        ipData := d.IPsMap[ip]
                        statServiceReq(d.IPsMap, ip, (uint64)(workerData.Latency))
                        ipData.ActualWorkers--
                        if ipData.DesiredWorders > ipData.ActualWorkers {
                            //If the IP desired workers is lower than actual workers
                            //Assign the IP to worker again
                            workerData.JobChannel <- types.Job{IP: ip}
                            ipData.ActualWorkers++
                        } else {
                            //If the IP desired workers is not lower than actual workers
                            //current IP needn't be assigned to this worker
                            //choose one IP from NeedAssignIPMap
                            choosedIP := d.chooseIPForWorker()
                            if (choosedIP != "") {
                                workerData.JobChannel <- types.Job{IP: choosedIP}
                                d.IPsMap[choosedIP].ActualWorkers++
                            }
                        }
                    } else {
                        // if desired worker is 0, it's already re assigned
                        if d.IPsMap[workerData.IP].DesiredWorders != 0 {
                            //service down, set the down server IP desired worker as 0
                            d.IPsMap[workerData.IP].DesiredWorders = 0

                            //service down, get the IPs from URL again
                            fmt.Println("refresh IP")
                            ips, err := getIps(d.URL)
                            if err != nil || len(ips) == 0 {
                                waited --
                                existError = true
                                break;
                            }

                            oldIPs := d.IPs
                            d.IPs = ips

                            //reassign desire workers to each IP
                            d.reAssignIPWorkers(oldIPs)
                        }

                        d.IPsMap[workerData.IP].ActualWorkers--

                        choosedIP := d.chooseIPForWorker()
                        if (choosedIP != "") {
                            workerData.JobChannel <- types.Job{IP: choosedIP}
                            d.IPsMap[choosedIP].ActualWorkers++
                        }
                    }
                }
            i++;

        }
    }

    for i:=0; i<waited; {
        select {
        case workerData := <-d.WorkerPool:
            if workerData.ServiceDown != true {
                statServiceReq(d.IPsMap, workerData.IP, (uint64)(workerData.Latency))
            }
            i++;
        }
    }
}

func isIPSame(ips []string, ipsMap map[string]*IPData) bool {
    for _, v := range ips {
        if ipData, ok := ipsMap[v]; !ok || ipData.DesiredWorders == 0 {
            return false
        }
    }
    return true
}

func (d *Dispatcher)chooseIPForWorker() string {
    if len(d.NeedAssignIPMap) != 0 {
        for key, v := range d.NeedAssignIPMap {
            if (v != 0) {
                d.NeedAssignIPMap[key] = v - 1
                return key
            }
        }
    }
    fmt.Println("ERROR: no IP needs more worker!")
    return ""
}

//TODO: re assign workers to IP base on average latency
func (d *Dispatcher)reAssignIPWorkers(oldIPs []string) {
    newIPsMap := map[string]bool{}
    for _, v := range d.IPs {
        newIPsMap[v] = true
    }
    for _, v := range oldIPs {
        if _, ok := newIPsMap[v]; !ok {
            //set old IP not in new IP list, desired worker as 0
            d.IPsMap[v].DesiredWorders = 0
        }
    }

    // average assign workers to each new IPs
    workersPerIp := d.maxWorkers/len(d.IPs)
    for _, v := range d.IPs {
        if ipData, ok := d.IPsMap[v]; ok {
            ipData.DesiredWorders = workersPerIp
            if ipData.DesiredWorders > ipData.ActualWorkers {
                d.NeedAssignIPMap[v] = ipData.DesiredWorders - ipData.ActualWorkers
            }
        } else {
            d.IPsMap[v] = &IPData {
                RequestCount: 0,
                DesiredWorders: workersPerIp,
                ActualWorkers: 0,
                MinLatency: ^uint64(0),
                MaxLatency: 0,
                AvgLatency: 0,
                TotalLatency: 0}
            d.NeedAssignIPMap[v] = workersPerIp
        }
    }
    if remain := d.maxWorkers - workersPerIp*len(d.IPs); remain > 0 {
        ipData := d.IPsMap[d.IPs[0]]
        ipData.DesiredWorders += remain
        if ipData.DesiredWorders > ipData.ActualWorkers {
            d.NeedAssignIPMap[d.IPs[0]] = ipData.DesiredWorders - ipData.ActualWorkers
        }
    }

}

func statServiceReq(ipsMap map[string]*IPData, ip string, latency uint64) {

    ipData := ipsMap[ip]

    ipData.RequestCount++
    ipData.TotalLatency += latency

    if latency < ipData.MinLatency {
        ipData.MinLatency = latency
    }

    if latency > ipData.MaxLatency {
        ipData.MaxLatency = latency
    }
}

func isIpStr(str string) bool {
    strSplit := strings.Split(str,".")
    if len(strSplit) != 4 {
        return false
    }
    for _,v := range strSplit {
        vInt, err := strconv.Atoi(v)
        if err != nil {
            return false
        }
        if vInt <= 255 && vInt >= 0 {
            continue
        } else {
            return false
        }
    }
    return true
}

func getIps(url string) ([]string,error) {

    var ips []string

    out, err := exec.Command("dig", url, "+short").Output()

    if err != nil {
        fmt.Printf("parse %s failed, err:%s", url, err)
        return nil, err
    }

    output := string(out[:])
    outputSplit := strings.Split(output,"\n")
    for _,v := range outputSplit {
        if len(v) != 0 {
            if (isIpStr(v)) {
                ips = append(ips, v)
                fmt.Printf("ip: %s\n",v)
            }
        }
    }
    if len(ips) == 0 {
        fmt.Println("No IPs obtained for", url)
    }

    return ips, nil
}