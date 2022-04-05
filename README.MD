   * [Client Load Balance](#client-load-balance)
      * [Build](#build)
      * [Usage](#usage)
      * [Implementation](#implementation)

# Client Load Balance
Suppose there is a web service, for example, the URL is http://www.ebay.com.

And behind the DNS, there are multiple servers providing the service, and each server has a different IP.

Client load balance to query the IPs behind the DNS, and send 100 requests to different IPs
according to client-side loadbalancing strategy in order to achieve better performance and lower
lantency.

The client load balance can handle IPs change behind DNS, IP unreachable. And also it will retry when request failure.

## Build
This Client Load Balance uses Go modules for dependency management and requires Go 1.18+ to build.

To build the binary:

``
make
``

Upon successful build the binary will be available in `build/clientLB`.

## Usage
./build/clientLB URL

You can also use ENV to set parameters, like MAX Workers, default MAX workers is 2*runtime.NumCPU()

For example:

```
./build/clientLB www.baidu.com
Total Workers: 8
ip: 36.152.44.96
ip: 36.152.44.95
Total duration: 944 ms
IP: 36.152.44.96 RequestCount: 52 MinLatency: 24 ms MaxLatency: 163 ms AvgLatency: 68 ms desiredWorker: 4 actualWorker: 4
IP: 36.152.44.95 RequestCount: 48 MinLatency: 34 ms MaxLatency: 182 ms AvgLatency: 76 ms desiredWorker: 4 actualWorker: 4
```

```
export MAX_WORKERS=12; ./build/clientLB www.baidu.com
Total Workers: 12
ip: 36.152.44.96
ip: 36.152.44.95
Total duration: 1156 ms
IP: 36.152.44.96 RequestCount: 55 MinLatency: 48 ms MaxLatency: 459 ms AvgLatency: 111 ms desiredWorker: 6 actualWorker: 6
IP: 36.152.44.95 RequestCount: 45 MinLatency: 24 ms MaxLatency: 564 ms AvgLatency: 138 ms desiredWorker: 6 actualWorker: 6
```

## Implementation

Client Load Balance will create a Scheduler, the scheduler will first query IPs from URL by dig tool, then it will create MaxWorkers Worker goroutine, for initial, each IP will be average assign MaxWorkers/len(IPs) worker, Scheduler will assign job(IP) to each worker, the worker will do http service request.

There is a Map stores the IP desired worker count, and actual worker count.

Normally if worker is done request succussfully, it will be assigned the same IP again, for quick request, quick do it again.

If the http request is failed(timeout and statusCode >500), a retry will be done for the request, if failed again, we consider the service IP is down.

When the worker finish http request, it will notify Scheduler, if request succesful, the scheduler check whether the desired workers
is larger then actual workers, if yes it will assign the same IP to the worker again. If service IP is down, the scheduler will query IPs again from URL, and reassign worker count to each new IPs.

There is also a refresh timer(default is 3s), if timeout, scheduler will query the IPs again, if IPs changed, scheduler will reassign worker count to each new IPs.

Reassign will re-set the desired worker count of each new IPs and old IPs, and the IPs need new worker will be stored in one map.

When worker request successful, and the scheduler check whether the desired workers is lower than actual workers, if yes, it will get one IP from the "IP needs new worker map", it will assign this new IP to the worker.

TODO: now the workers are average assigned to each IP, we can based on avarage latency to assign workers to IPs.


