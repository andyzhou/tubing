package define

//request method
const (
	ReqMethodOfGet  = "GET"
	ReqMethodOfPost = "POST"
)

//default value
const (
	AppName                   = "tubing"
	DefaultConnectTicker      = 0.1 //xx seconds
	DefaultReadMsgTicker      = 0.1 //xx seconds
	DefaultSendMsgTicker      = 0.1 //xx seconds
	DefaultDialTimeOut        = 5   //xx seconds
	DefaultHeartBeatRate      = 120 //xx seconds
	DefaultBuckets            = 5   //xx buckets
	DefaultReadDataRate       = float64(0.1)
	DefaultReadTimeout		  = float64(0.1)
	DefaultTenThousandPercent = 10000
	DefaultGcOptRate          = 50 //xx percent
)
