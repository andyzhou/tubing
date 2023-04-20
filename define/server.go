package define

//websocket
const (
	WebSocketRoot = "/ws"
	WebSocketBufferSize = 1024 * 5
)

//param
const (
	QueryParaOfContentType = "type"
	QueryParaOfSession = "session"
)

//request method
const (
	ReqMethodOfGet = "GET"
	ReqMethodOfPost = "POST"
)

//request path
const (
	AnyPath = "any"
)

//default value
const (
	AppName = "tubing"
	ClientHeartBeatRate = 5
	ServerHeartBeatRate = 30
)

const (
	HeaderOfContentType = "Content-Type"
	ContentTypeOfJson = "application/json"
	ContentTypeOfOctet = "application/octet"
)