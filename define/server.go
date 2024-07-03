package define

//request method
const (
	ReqMethodOfGet = "GET"
	ReqMethodOfPost = "POST"
)

//default value
const (
	AppName = "tubing"
	DefaultConnectTicker = 0.1 //xx seconds
	DefaultReadMsgTicker = 0.1 //xx seconds
	DefaultSendMsgTicker = 0.1 //xx seconds
	DefaultDialTimeOut = 5 //xx seconds
	DefaultHeartBeatRate = 60 //xx seconds
	DefaultBuckets = 5 //xx buckets
	DefaultWorkers = 5 //xx workers for one bucket
	DefaultReadDataRate = float64(0.2)
)
